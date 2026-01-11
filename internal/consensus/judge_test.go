package consensus

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/johnayoung/llm-consensus/internal/provider"
)

func TestJudge_Synthesize(t *testing.T) {
	tests := []struct {
		name      string
		responses []provider.Response
		judge     func(ctx context.Context, req provider.Request) (provider.Response, error)
		wantErr   bool
		check     func(t *testing.T, result string)
	}{
		{
			name:      "empty responses returns error",
			responses: []provider.Response{},
			wantErr:   true,
		},
		{
			name: "single response returns content directly",
			responses: []provider.Response{
				{Model: "model-a", Content: "single answer", Provider: "test"},
			},
			check: func(t *testing.T, result string) {
				if result != "single answer" {
					t.Errorf("got %q, want %q", result, "single answer")
				}
			},
		},
		{
			name: "multiple responses calls judge",
			responses: []provider.Response{
				{Model: "model-a", Content: "answer a", Provider: "test"},
				{Model: "model-b", Content: "answer b", Provider: "test"},
			},
			judge: func(ctx context.Context, req provider.Request) (provider.Response, error) {
				// Verify prompt contains both responses
				if !strings.Contains(req.Prompt, "answer a") || !strings.Contains(req.Prompt, "answer b") {
					t.Error("judge prompt missing model responses")
				}
				return provider.Response{Content: "synthesized consensus"}, nil
			},
			check: func(t *testing.T, result string) {
				if result != "synthesized consensus" {
					t.Errorf("got %q, want %q", result, "synthesized consensus")
				}
			},
		},
		{
			name: "judge failure propagates error",
			responses: []provider.Response{
				{Model: "model-a", Content: "answer a"},
				{Model: "model-b", Content: "answer b"},
			},
			judge: func(ctx context.Context, req provider.Request) (provider.Response, error) {
				return provider.Response{}, errors.New("judge api error")
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var p provider.Provider
			if tt.judge != nil {
				p = provider.ProviderFunc(tt.judge)
			} else {
				p = provider.ProviderFunc(func(ctx context.Context, req provider.Request) (provider.Response, error) {
					return provider.Response{}, nil
				})
			}

			judge := NewJudge(p, "test-model")
			result, err := judge.Synthesize(context.Background(), "original prompt", tt.responses)

			if tt.wantErr {
				if err == nil {
					t.Error("expected error, got nil")
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if tt.check != nil {
				tt.check(t, result)
			}
		})
	}
}

func TestJudge_PromptTemplate(t *testing.T) {
	var capturedPrompt string

	p := provider.ProviderFunc(func(ctx context.Context, req provider.Request) (provider.Response, error) {
		capturedPrompt = req.Prompt
		return provider.Response{Content: "consensus"}, nil
	})

	judge := NewJudge(p, "judge-model")
	responses := []provider.Response{
		{Model: "gpt-4o", Content: "GPT says hello", Provider: "openai", Latency: 100 * time.Millisecond},
		{Model: "claude-sonnet", Content: "Claude says hi", Provider: "anthropic", Latency: 150 * time.Millisecond},
	}

	_, err := judge.Synthesize(context.Background(), "Say hello", responses)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify template expansion
	checks := []string{
		"Say hello",         // Original prompt
		"gpt-4o",            // Model name
		"claude-sonnet",     // Model name
		"GPT says hello",    // Response content
		"Claude says hi",    // Response content
		"openai",            // Provider name
		"anthropic",         // Provider name
	}

	for _, check := range checks {
		if !strings.Contains(capturedPrompt, check) {
			t.Errorf("prompt missing %q", check)
		}
	}
}
