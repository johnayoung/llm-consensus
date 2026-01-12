package provider

import (
	"context"
	"time"
)

// StreamCallback is called for each chunk of streamed content.
// The chunk parameter contains the incremental text received.
type StreamCallback func(chunk string)

// Provider abstracts LLM API interactions.
type Provider interface {
	// Query sends a prompt and returns the complete response.
	Query(ctx context.Context, req Request) (Response, error)

	// QueryStream sends a prompt and streams the response via callback.
	// The callback is invoked for each chunk of text received.
	// Returns the complete response when finished.
	QueryStream(ctx context.Context, req Request, callback StreamCallback) (Response, error)
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

func (f ProviderFunc) QueryStream(ctx context.Context, req Request, callback StreamCallback) (Response, error) {
	// For simple function adapters, just call Query and invoke callback with full content
	resp, err := f(ctx, req)
	if err != nil {
		return resp, err
	}
	if callback != nil {
		callback(resp.Content)
	}
	return resp, nil
}
