package provider

import (
	"fmt"
	"sync"
)

// Registry maps model names to their providers.
// Thread-safe for concurrent access during queries.
type Registry struct {
	mu        sync.RWMutex
	providers map[string]Provider
}

// NewRegistry creates an empty registry.
func NewRegistry() *Registry {
	return &Registry{
		providers: make(map[string]Provider),
	}
}

// Register associates a model name with a provider.
// Safe to call concurrently.
func (r *Registry) Register(model string, p Provider) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.providers[model] = p
}

// Get retrieves the provider for a model.
// Returns an error if the model is not registered.
func (r *Registry) Get(model string) (Provider, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	p, ok := r.providers[model]
	if !ok {
		return nil, fmt.Errorf("unknown model: %s", model)
	}
	return p, nil
}

// Models returns all registered model names.
func (r *Registry) Models() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	models := make([]string, 0, len(r.providers))
	for m := range r.providers {
		models = append(models, m)
	}
	return models
}
