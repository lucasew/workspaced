package tool

import (
	"fmt"
	"sync"
	"workspaced/pkg/tool/provider"
)

var (
	mu        sync.RWMutex
	providers = make(map[string]provider.Provider)
)

// RegisterProvider adds a new tool provider to the global registry.
// This function is thread-safe and is typically called during init() by provider implementations.
func RegisterProvider(p provider.Provider) {
	mu.Lock()
	defer mu.Unlock()
	providers[p.ID()] = p
}

// GetProvider retrieves a registered provider by its ID.
// Returns an error if no provider is found with the given ID.
// This function is thread-safe.
func GetProvider(id string) (provider.Provider, error) {
	mu.RLock()
	defer mu.RUnlock()
	p, ok := providers[id]
	if !ok {
		return nil, fmt.Errorf("provider not found: %s", id)
	}
	return p, nil
}
