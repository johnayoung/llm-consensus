// /cmd/model-registry-sync/main.go
package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"sort"
	"strings"
	"time"
)

type ModelRecord struct {
	Source        string           `json:"source"`                   // "openai" | "openrouter"
	ID            string           `json:"id"`                       // provider model id
	Name          string           `json:"name,omitempty"`           // if known
	ContextLength int              `json:"context_length,omitempty"` // if known
	Pricing       *OpenRouterPrice `json:"pricing,omitempty"`        // if known
	Raw           any              `json:"raw,omitempty"`            // optional debugging
}

type OpenAIListModelsResponse struct {
	Object string        `json:"object"`
	Data   []OpenAIModel `json:"data"`
}

type OpenAIModel struct {
	ID      string `json:"id"`
	Object  string `json:"object"`
	Created int64  `json:"created"`
	OwnedBy string `json:"owned_by"`
}

type OpenRouterListModelsResponse struct {
	Data []OpenRouterModel `json:"data"`
}

type OpenRouterModel struct {
	ID            string          `json:"id"`
	CanonicalSlug string          `json:"canonical_slug"`
	Name          string          `json:"name"`
	Created       int64           `json:"created"`
	ContextLength int             `json:"context_length"`
	Pricing       OpenRouterPrice `json:"pricing"`
	Description   string          `json:"description"`
}

type OpenRouterPrice struct {
	Prompt     string `json:"prompt"`
	Completion string `json:"completion"`
	Request    string `json:"request"`
	Image      string `json:"image"`
}

func main() {
	var (
		outPath        string
		includeRaw     bool
		openaiEnabled  bool
		orEnabled      bool
		timeoutSeconds int
	)
	flag.StringVar(&outPath, "out", "", "output file path (defaults to stdout)")
	flag.BoolVar(&includeRaw, "raw", false, "include raw provider objects in output (debugging)")
	flag.BoolVar(&openaiEnabled, "openai", true, "fetch OpenAI models (requires OPENAI_API_KEY)")
	flag.BoolVar(&orEnabled, "openrouter", true, "fetch OpenRouter models (uses OPENROUTER_API_KEY if set)")
	flag.IntVar(&timeoutSeconds, "timeout", 20, "HTTP timeout in seconds")
	flag.Parse()

	ctx := context.Background()
	client := &http.Client{Timeout: time.Duration(timeoutSeconds) * time.Second}

	var all []ModelRecord
	var errs []error

	if openaiEnabled {
		recs, err := fetchOpenAIModels(ctx, client, includeRaw)
		if err != nil {
			errs = append(errs, fmt.Errorf("openai: %w", err))
		} else {
			all = append(all, recs...)
		}
	}

	if orEnabled {
		recs, err := fetchOpenRouterModels(ctx, client, includeRaw)
		if err != nil {
			errs = append(errs, fmt.Errorf("openrouter: %w", err))
		} else {
			all = append(all, recs...)
		}
	}

	// Stable ordering
	sort.Slice(all, func(i, j int) bool {
		if all[i].Source == all[j].Source {
			return all[i].ID < all[j].ID
		}
		return all[i].Source < all[j].Source
	})

	payload, err := json.MarshalIndent(all, "", "  ")
	if err != nil {
		fatal(err)
	}

	if outPath == "" {
		_, _ = os.Stdout.Write(payload)
		_, _ = os.Stdout.Write([]byte("\n"))
	} else {
		if err := os.WriteFile(outPath, payload, 0o644); err != nil {
			fatal(err)
		}
	}

	// Non-fatal: show fetch errors at the end (so you still get partial output).
	if len(errs) > 0 {
		_, _ = fmt.Fprintln(os.Stderr, "\nWARN: some sources failed:")
		for _, e := range errs {
			_, _ = fmt.Fprintln(os.Stderr, " -", e.Error())
		}
	}
}

func fetchOpenAIModels(ctx context.Context, client *http.Client, includeRaw bool) ([]ModelRecord, error) {
	apiKey := strings.TrimSpace(os.Getenv("OPENAI_API_KEY"))
	if apiKey == "" {
		return nil, errors.New("OPENAI_API_KEY not set")
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "https://api.openai.com/v1/models", nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+apiKey)

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("http %d: %s", resp.StatusCode, truncate(string(body), 600))
	}

	var parsed OpenAIListModelsResponse
	if err := json.Unmarshal(body, &parsed); err != nil {
		return nil, fmt.Errorf("unmarshal: %w; body=%s", err, truncate(string(body), 600))
	}

	out := make([]ModelRecord, 0, len(parsed.Data))
	for _, m := range parsed.Data {
		rec := ModelRecord{
			Source: "openai",
			ID:     m.ID,
		}
		if includeRaw {
			rec.Raw = m
		}
		out = append(out, rec)
	}
	return out, nil
}

func fetchOpenRouterModels(ctx context.Context, client *http.Client, includeRaw bool) ([]ModelRecord, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "https://openrouter.ai/api/v1/models", nil)
	if err != nil {
		return nil, err
	}

	// Docs show Bearer auth; if you have a key, include it.
	// Some setups may still allow unauthenticated access; this keeps it flexible.
	if key := strings.TrimSpace(os.Getenv("OPENROUTER_API_KEY")); key != "" {
		req.Header.Set("Authorization", "Bearer "+key)
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("http %d: %s", resp.StatusCode, truncate(string(body), 600))
	}

	var parsed OpenRouterListModelsResponse
	if err := json.Unmarshal(body, &parsed); err != nil {
		return nil, fmt.Errorf("unmarshal: %w; body=%s", err, truncate(string(body), 600))
	}

	out := make([]ModelRecord, 0, len(parsed.Data))
	for _, m := range parsed.Data {
		price := m.Pricing // copy
		rec := ModelRecord{
			Source:        "openrouter",
			ID:            m.ID,
			Name:          m.Name,
			ContextLength: m.ContextLength,
			Pricing:       &price,
		}
		if includeRaw {
			rec.Raw = m
		}
		out = append(out, rec)
	}
	return out, nil
}

func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max] + "…"
}

func fatal(err error) {
	_, _ = fmt.Fprintln(os.Stderr, "ERROR:", err.Error())
	os.Exit(1)
}
