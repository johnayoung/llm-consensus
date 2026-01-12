package runner

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/johnayoung/llm-consensus/internal/provider"
	"golang.org/x/sync/errgroup"
)

// Callbacks for progress reporting during model queries.
type Callbacks struct {
	OnModelStart    func(model string)
	OnModelStream   func(model string, chunk string)
	OnModelComplete func(model string)
	OnModelError    func(model string, err error)
}

// Result contains the outcomes of querying multiple models.
type Result struct {
	Responses    []provider.Response
	Warnings     []string
	FailedModels []string
}

// Runner orchestrates parallel LLM queries.
type Runner struct {
	registry  *provider.Registry
	timeout   time.Duration
	callbacks *Callbacks
}

// New creates a runner with the given registry and per-model timeout.
func New(registry *provider.Registry, timeout time.Duration) *Runner {
	return &Runner{
		registry: registry,
		timeout:  timeout,
	}
}

// WithCallbacks sets progress callbacks for the runner.
func (r *Runner) WithCallbacks(cb *Callbacks) *Runner {
	r.callbacks = cb
	return r
}

// Run queries all models concurrently and collects results.
// Uses best-effort strategy: partial failures don't abort the run.
func (r *Runner) Run(ctx context.Context, models []string, prompt string) (*Result, error) {
	var (
		mu           sync.Mutex
		responses    []provider.Response
		warnings     []string
		failedModels []string
	)

	g, ctx := errgroup.WithContext(ctx)

	for _, model := range models {
		g.Go(func() error {
			// Per-model timeout
			modelCtx, cancel := context.WithTimeout(ctx, r.timeout)
			defer cancel()

			// Notify start
			if r.callbacks != nil && r.callbacks.OnModelStart != nil {
				r.callbacks.OnModelStart(model)
			}

			p, err := r.registry.Get(model)
			if err != nil {
				mu.Lock()
				warnings = append(warnings, fmt.Sprintf("%s: %v", model, err))
				failedModels = append(failedModels, model)
				mu.Unlock()
				if r.callbacks != nil && r.callbacks.OnModelError != nil {
					r.callbacks.OnModelError(model, err)
				}
				return nil // best effort: don't fail entire run
			}

			// Use streaming query with callback
			streamCallback := func(chunk string) {
				if r.callbacks != nil && r.callbacks.OnModelStream != nil {
					r.callbacks.OnModelStream(model, chunk)
				}
			}

			resp, err := p.QueryStream(modelCtx, provider.Request{
				Model:  model,
				Prompt: prompt,
			}, streamCallback)

			mu.Lock()
			defer mu.Unlock()

			if err != nil {
				warnings = append(warnings, fmt.Sprintf("%s: %v", model, err))
				failedModels = append(failedModels, model)
				if r.callbacks != nil && r.callbacks.OnModelError != nil {
					r.callbacks.OnModelError(model, err)
				}
				return nil // best effort
			}

			responses = append(responses, resp)
			if r.callbacks != nil && r.callbacks.OnModelComplete != nil {
				r.callbacks.OnModelComplete(model)
			}
			return nil
		})
	}

	// Wait for all goroutines to complete
	if err := g.Wait(); err != nil {
		return nil, err
	}

	if len(responses) == 0 {
		return nil, errors.New("all models failed: " + fmt.Sprintf("%v", warnings))
	}

	return &Result{
		Responses:    responses,
		Warnings:     warnings,
		FailedModels: failedModels,
	}, nil
}
