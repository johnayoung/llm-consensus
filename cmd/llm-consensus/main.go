package main

import (
	"bufio"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"runtime/debug"
	"strings"
	"syscall"
	"time"

	"github.com/johnayoung/llm-consensus/internal/consensus"
	"github.com/johnayoung/llm-consensus/internal/output"
	"github.com/johnayoung/llm-consensus/internal/provider"
	"github.com/johnayoung/llm-consensus/internal/runner"
	"github.com/johnayoung/llm-consensus/internal/ui"
)

// Version information set via ldflags at build time.
var (
	version = "dev"
	commit  = "none"
	date    = "unknown"
)

const (
	defaultJudge   = "gpt-5.2-pro-2025-12-11"
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
	"gpt-5.2-2025-12-11":     ProviderOpenAI,
	"gpt-5.2-pro-2025-12-11": ProviderOpenAI,

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
	dataDir string
	timeout time.Duration
	prompt  string
	quiet   bool
	json    bool
	noSave  bool
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

	// Determine if we should show UI (interactive terminal and not quiet)
	// Show UI even when --output is specified (progress goes to stderr, JSON to file)
	showUI := ui.IsTerminal(os.Stderr) && !cfg.quiet && !cfg.json
	startTime := time.Now()

	// Initialize providers based on requested models
	registry, err := initRegistry(cfg.models, cfg.judge)
	if err != nil {
		return err
	}

	if showUI {
		ui.PrintHeader(os.Stderr, cfg.prompt)
		ui.PrintPhase(os.Stderr, "Querying models...")
		fmt.Fprintln(os.Stderr) // blank line for progress display
	}

	// Setup progress display
	progress := ui.NewProgress(os.Stderr, cfg.models, !showUI)
	progress.Start()

	// Create runner with timeout and callbacks
	r := runner.New(registry, cfg.timeout)
	r.WithCallbacks(&runner.Callbacks{
		OnModelStart: func(model string) {
			progress.ModelStarted(model)
		},
		OnModelStream: func(model, chunk string) {
			progress.ModelStreaming(model, chunk)
		},
		OnModelComplete: func(model string) {
			progress.ModelCompleted(model)
		},
		OnModelError: func(model string, err error) {
			progress.ModelFailed(model, err)
		},
	})

	// Execute queries in parallel with streaming
	result, err := r.Run(ctx, cfg.models, cfg.prompt)

	// Stop progress display
	progress.Stop()

	if err != nil {
		return fmt.Errorf("running queries: %w", err)
	}

	if showUI {
		ui.PrintSuccess(os.Stderr, fmt.Sprintf("Received responses from %d models", len(result.Responses)))
		fmt.Fprintln(os.Stderr)
		ui.PrintPhase(os.Stderr, "Synthesizing consensus...")
		fmt.Fprintln(os.Stderr)
	}

	// Get consensus from judge with streaming
	judgeProvider, err := registry.Get(cfg.judge)
	if err != nil {
		return fmt.Errorf("judge model %s: %w", cfg.judge, err)
	}

	judge := consensus.NewJudge(judgeProvider, cfg.judge)

	// Setup judge progress
	judgeProgress := ui.NewProgress(os.Stderr, []string{cfg.judge}, !showUI)
	judgeProgress.Start()
	judgeProgress.ModelStarted(cfg.judge)

	consensusResp, err := judge.SynthesizeStream(ctx, cfg.prompt, result.Responses, func(chunk string) {
		judgeProgress.ModelStreaming(cfg.judge, chunk)
	})

	judgeProgress.ModelCompleted(cfg.judge)
	judgeProgress.Stop()

	if err != nil {
		return fmt.Errorf("consensus synthesis: %w", err)
	}

	if showUI {
		ui.PrintSuccess(os.Stderr, "Consensus reached!")
	}

	// Format output
	out := output.Result{
		Prompt:       cfg.prompt,
		Responses:    result.Responses,
		Consensus:    consensusResp,
		Judge:        cfg.judge,
		Warnings:     result.Warnings,
		FailedModels: result.FailedModels,
	}

	// Determine output path
	var outputPath string
	if cfg.output != "" {
		// Explicit output path specified
		outputPath = cfg.output
	} else if !cfg.json && !cfg.noSave {
		// Auto-save to data/<run-id>/
		runID := generateRunID()
		runDir := filepath.Join(cfg.dataDir, runID)
		if err := os.MkdirAll(runDir, 0755); err != nil {
			return fmt.Errorf("creating run directory: %w", err)
		}
		outputPath = filepath.Join(runDir, "result.json")

		// Also save the prompt for reference
		promptPath := filepath.Join(runDir, "prompt.txt")
		if err := os.WriteFile(promptPath, []byte(cfg.prompt), 0644); err != nil {
			// Non-fatal, just warn
			if showUI {
				ui.PrintError(os.Stderr, fmt.Sprintf("Failed to save prompt: %v", err))
			}
		}

		// Save consensus as markdown for easy reading
		consensusPath := filepath.Join(runDir, "consensus.md")
		if err := os.WriteFile(consensusPath, []byte(consensusResp), 0644); err != nil {
			if showUI {
				ui.PrintError(os.Stderr, fmt.Sprintf("Failed to save consensus: %v", err))
			}
		}
	}

	// Write output
	if outputPath != "" {
		// Write JSON to file
		w, err := os.Create(outputPath)
		if err != nil {
			return fmt.Errorf("creating output file: %w", err)
		}
		defer w.Close()

		enc := json.NewEncoder(w)
		enc.SetIndent("", "  ")
		if err := enc.Encode(out); err != nil {
			return err
		}

		if showUI {
			fmt.Fprintln(os.Stderr)
			ui.PrintSuccess(os.Stderr, fmt.Sprintf("Run saved to %s", filepath.Dir(outputPath)))
		}
	} else if cfg.json {
		// JSON to stdout (no auto-save)
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(out)
	} else if showUI {
		// Pretty print to terminal (already saved above if auto-save enabled)
		fmt.Fprintln(os.Stderr)

		// Print individual model responses
		for _, resp := range result.Responses {
			ui.PrintModelResponse(os.Stderr, resp.Model, resp.Provider, resp.Content, resp.Latency)
		}

		// Print consensus
		ui.PrintConsensus(os.Stderr, consensusResp)

		// Print summary
		ui.PrintSummary(os.Stderr,
			len(cfg.models),
			len(result.Responses),
			len(result.FailedModels),
			time.Since(startTime))

		// Print warnings if any
		if len(result.Warnings) > 0 {
			fmt.Fprintln(os.Stderr)
			for _, w := range result.Warnings {
				ui.PrintError(os.Stderr, w)
			}
		}
	} else {
		// Non-interactive: JSON to stdout
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(out)
	}

	return nil
}

// generateRunID creates a unique run identifier using timestamp + random suffix.
// Format: 20260112-143052-a1b2c3
func generateRunID() string {
	timestamp := time.Now().Format("20060102-150405")
	suffix := make([]byte, 3)
	rand.Read(suffix)
	return fmt.Sprintf("%s-%s", timestamp, hex.EncodeToString(suffix))
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
		dataDir     string
		timeout     int
		quiet       bool
		jsonOutput  bool
		noSave      bool
		showVersion bool
	)

	flag.StringVar(&modelsStr, "models", "", "Comma-separated list of models to query (required)")
	flag.StringVar(&judge, "judge", defaultJudge, "Model to use for consensus synthesis")
	flag.StringVar(&file, "file", "", "Read prompt from file")
	flag.StringVar(&outputPath, "output", "", "Write JSON output to specific file (overrides auto-save)")
	flag.StringVar(&dataDir, "data-dir", "data", "Directory for auto-saved runs")
	flag.IntVar(&timeout, "timeout", 120, "Per-model timeout in seconds")
	flag.BoolVar(&quiet, "quiet", false, "Suppress progress output")
	flag.BoolVar(&quiet, "q", false, "Suppress progress output (shorthand)")
	flag.BoolVar(&jsonOutput, "json", false, "Output JSON to stdout (no interactive display, no auto-save)")
	flag.BoolVar(&noSave, "no-save", false, "Don't auto-save results to data directory")
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
		dataDir: dataDir,
		timeout: time.Duration(timeout) * time.Second,
		quiet:   quiet,
		json:    jsonOutput,
		noSave:  noSave,
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
