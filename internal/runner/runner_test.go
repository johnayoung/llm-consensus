package runner

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/johnayoung/llm-consensus/internal/provider"
)

func TestRunner_Run(t *testing.T) {
	tests := []struct {
		name         string
		models       []string
		setup        func(*provider.Registry)
		wantRespLen  int
		wantWarnLen  int
		wantFailLen  int
		wantErr      bool
	}{
		{
			name:   "all models succeed",
			models: []string{"model-a", "model-b"},
			setup: func(r *provider.Registry) {
				r.Register("model-a", provider.ProviderFunc(func(ctx context.Context, req provider.Request) (provider.Response, error) {
					return provider.Response{Model: "model-a", Content: "response a", Provider: "test"}, nil
				}))
				r.Register("model-b", provider.ProviderFunc(func(ctx context.Context, req provider.Request) (provider.Response, error) {
					return provider.Response{Model: "model-b", Content: "response b", Provider: "test"}, nil
				}))
			},
			wantRespLen: 2,
			wantWarnLen: 0,
			wantFailLen: 0,
		},
		{
			name:   "partial failure - one model fails",
			models: []string{"model-a", "model-b"},
			setup: func(r *provider.Registry) {
				r.Register("model-a", provider.ProviderFunc(func(ctx context.Context, req provider.Request) (provider.Response, error) {
					return provider.Response{}, errors.New("api error")
				}))
				r.Register("model-b", provider.ProviderFunc(func(ctx context.Context, req provider.Request) (provider.Response, error) {
					return provider.Response{Model: "model-b", Content: "response b", Provider: "test"}, nil
				}))
			},
			wantRespLen: 1,
			wantWarnLen: 1,
			wantFailLen: 1,
		},
		{
			name:   "all models fail",
			models: []string{"model-a", "model-b"},
			setup: func(r *provider.Registry) {
				r.Register("model-a", provider.ProviderFunc(func(ctx context.Context, req provider.Request) (provider.Response, error) {
					return provider.Response{}, errors.New("error a")
				}))
				r.Register("model-b", provider.ProviderFunc(func(ctx context.Context, req provider.Request) (provider.Response, error) {
					return provider.Response{}, errors.New("error b")
				}))
			},
			wantErr: true,
		},
		{
			name:   "unregistered model",
			models: []string{"unknown-model"},
			setup:  func(r *provider.Registry) {},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			reg := provider.NewRegistry()
			tt.setup(reg)

			runner := New(reg, 5*time.Second)
			result, err := runner.Run(context.Background(), tt.models, "test prompt")

			if tt.wantErr {
				if err == nil {
					t.Error("expected error, got nil")
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if len(result.Responses) != tt.wantRespLen {
				t.Errorf("got %d responses, want %d", len(result.Responses), tt.wantRespLen)
			}

			if len(result.Warnings) != tt.wantWarnLen {
				t.Errorf("got %d warnings, want %d", len(result.Warnings), tt.wantWarnLen)
			}

			if len(result.FailedModels) != tt.wantFailLen {
				t.Errorf("got %d failed models, want %d", len(result.FailedModels), tt.wantFailLen)
			}
		})
	}
}

func TestRunner_Timeout(t *testing.T) {
	reg := provider.NewRegistry()
	reg.Register("slow-model", provider.ProviderFunc(func(ctx context.Context, req provider.Request) (provider.Response, error) {
		select {
		case <-ctx.Done():
			return provider.Response{}, ctx.Err()
		case <-time.After(10 * time.Second):
			return provider.Response{Model: "slow-model", Content: "too slow"}, nil
		}
	}))

	runner := New(reg, 100*time.Millisecond)
	result, err := runner.Run(context.Background(), []string{"slow-model"}, "test")

	if err == nil {
		t.Fatal("expected error due to all models timing out")
	}

	// Error should indicate timeout
	if result != nil && len(result.FailedModels) == 0 {
		t.Error("expected failed models to include slow-model")
	}
}
