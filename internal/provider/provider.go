package provider

import (
	"context"
	"time"
)

// Provider abstracts LLM API interactions.
// Single-method interface enables easy mocking and composition.
type Provider interface {
	Query(ctx context.Context, req Request) (Response, error)
}

// Request contains all inputs for an LLM query.
type Request struct {
	Model  string
	Prompt string
}

// Response contains the result of an LLM query.
type Response struct {
	Model    string        `json:"model"`
	Content  string        `json:"content"`
	Provider string        `json:"provider"`
	Latency  time.Duration `json:"latency_ms"`
}

// ProviderFunc allows functions to implement Provider (adapter pattern).
// Useful for testing and simple inline implementations.
type ProviderFunc func(ctx context.Context, req Request) (Response, error)

func (f ProviderFunc) Query(ctx context.Context, req Request) (Response, error) {
	return f(ctx, req)
}
