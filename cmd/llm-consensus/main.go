package main

import (
	"bufio"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"runtime/debug"
	"strings"
	"syscall"
	"time"

	"github.com/johnayoung/llm-consensus/internal/consensus"
	"github.com/johnayoung/llm-consensus/internal/output"
	"github.com/johnayoung/llm-consensus/internal/provider"
	"github.com/johnayoung/llm-consensus/internal/runner"
)

// Version information set via ldflags at build time.
var (
	version = "dev"
	commit  = "none"
	date    = "unknown"
)

const (
	defaultJudge   = "gpt-5.2-pro"
	defaultTimeout = 120 * time.Second
)

// ProviderType identifies which LLM provider to use.
type ProviderType int

const (
	ProviderOpenAI ProviderType = iota
	ProviderAnthropic
	ProviderGoogle
)

// Known models mapped to their providers.
// Add new models here as they become available.
var knownModels = map[string]ProviderType{
	// OpenAI
	"gpt-5.2-2025-12-11": ProviderOpenAI,

	// Anthropic (use full dated model names)
	"claude-sonnet-4-5": ProviderAnthropic,
	"claude-haiku-4-5":  ProviderAnthropic,
	"claude-opus-4-5":   ProviderAnthropic,

	// Google
	"gemini-3-pro-preview": ProviderGoogle,
}

type config struct {
	models  []string
	judge   string
	file    string
	output  string
	timeout time.Duration
	prompt  string
}

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}

func run() error {
	cfg, err := parseFlags()
	if err != nil {
		return err
	}

	// Setup context with signal handling for graceful shutdown
	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	// Initialize providers based on requested models
	registry, err := initRegistry(cfg.models, cfg.judge)
	if err != nil {
		return err
	}

	// Create runner with timeout
	r := runner.New(registry, cfg.timeout)

	// Execute queries in parallel
	result, err := r.Run(ctx, cfg.models, cfg.prompt)
	if err != nil {
		return fmt.Errorf("running queries: %w", err)
	}

	// Get consensus from judge
	judgeProvider, err := registry.Get(cfg.judge)
	if err != nil {
		return fmt.Errorf("judge model %s: %w", cfg.judge, err)
	}

	judge := consensus.NewJudge(judgeProvider, cfg.judge)
	consensusResp, err := judge.Synthesize(ctx, cfg.prompt, result.Responses)
	if err != nil {
		return fmt.Errorf("consensus synthesis: %w", err)
	}

	// Format and output result
	out := output.Result{
		Prompt:       cfg.prompt,
		Responses:    result.Responses,
		Consensus:    consensusResp,
		Judge:        cfg.judge,
		Warnings:     result.Warnings,
		FailedModels: result.FailedModels,
	}

	// Write to file or stdout
	var w *os.File
	if cfg.output != "" {
		var err error
		w, err = os.Create(cfg.output)
		if err != nil {
			return fmt.Errorf("creating output file: %w", err)
		}
		defer w.Close()
	} else {
		w = os.Stdout
	}

	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(out)
}

// getVersion returns the version string, using build info as fallback.
func getVersion() string {
	if version != "dev" {
		return version
	}
	if info, ok := debug.ReadBuildInfo(); ok && info.Main.Version != "(devel)" {
		return info.Main.Version
	}
	return "dev"
}

func parseFlags() (*config, error) {
	var (
		modelsStr   string
		judge       string
		file        string
		outputPath  string
		timeout     int
		showVersion bool
	)

	flag.StringVar(&modelsStr, "models", "", "Comma-separated list of models to query (required)")
	flag.StringVar(&judge, "judge", defaultJudge, "Model to use for consensus synthesis")
	flag.StringVar(&file, "file", "", "Read prompt from file")
	flag.StringVar(&outputPath, "output", "", "Write JSON output to file (default: stdout)")
	flag.IntVar(&timeout, "timeout", 120, "Per-model timeout in seconds")
	flag.BoolVar(&showVersion, "version", false, "Print version information and exit")
	flag.Parse()

	if showVersion {
		fmt.Printf("llm-consensus %s\n", getVersion())
		fmt.Printf("  commit: %s\n", commit)
		fmt.Printf("  built:  %s\n", date)
		os.Exit(0)
	}

	if modelsStr == "" {
		return nil, fmt.Errorf("--models flag is required")
	}

	models := strings.Split(modelsStr, ",")
	for i := range models {
		models[i] = strings.TrimSpace(models[i])
	}

	cfg := &config{
		models:  models,
		judge:   judge,
		file:    file,
		output:  outputPath,
		timeout: time.Duration(timeout) * time.Second,
	}

	// Get prompt from: positional arg > file > stdin
	prompt, err := getPrompt(flag.Args(), file)
	if err != nil {
		return nil, err
	}
	cfg.prompt = prompt

	return cfg, nil
}

func getPrompt(args []string, file string) (string, error) {
	// Priority 1: Positional argument
	if len(args) > 0 {
		return strings.Join(args, " "), nil
	}

	// Priority 2: File flag
	if file != "" {
		data, err := os.ReadFile(file)
		if err != nil {
			return "", fmt.Errorf("reading prompt file: %w", err)
		}
		return strings.TrimSpace(string(data)), nil
	}

	// Priority 3: Stdin (if not a terminal)
	stat, _ := os.Stdin.Stat()
	if (stat.Mode() & os.ModeCharDevice) == 0 {
		scanner := bufio.NewScanner(os.Stdin)
		var lines []string
		for scanner.Scan() {
			lines = append(lines, scanner.Text())
		}
		if err := scanner.Err(); err != nil {
			return "", fmt.Errorf("reading stdin: %w", err)
		}
		return strings.Join(lines, "\n"), nil
	}

	return "", fmt.Errorf("no prompt provided: use positional argument, --file, or pipe to stdin")
}

func initRegistry(models []string, judge string) (*provider.Registry, error) {
	registry := provider.NewRegistry()

	// Collect all unique models (including judge)
	needed := make(map[string]bool)
	for _, m := range models {
		needed[m] = true
	}
	needed[judge] = true

	// Initialize providers for each model
	for model := range needed {
		p, err := createProvider(model)
		if err != nil {
			return nil, fmt.Errorf("initializing provider for %s: %w", model, err)
		}
		registry.Register(model, p)
	}

	return registry, nil
}

func createProvider(model string) (provider.Provider, error) {
	providerType, ok := knownModels[model]
	if !ok {
		// List available models for helpful error message
		var available []string
		for m := range knownModels {
			available = append(available, m)
		}
		return nil, fmt.Errorf("unknown model %q; available models: %v", model, available)
	}

	switch providerType {
	case ProviderOpenAI:
		return provider.NewOpenAI()
	case ProviderAnthropic:
		return provider.NewAnthropic()
	case ProviderGoogle:
		return provider.NewGoogle()
	default:
		return nil, fmt.Errorf("unhandled provider type for model %s", model)
	}
}
