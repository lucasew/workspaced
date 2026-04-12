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

// RegisterProvider registers an artifact provider (like GitHub Releases, or specific registries)
// for use by the Manager. It must be called during initialization (typically via a prelude)
// to make the provider available for tool installation. Thread-safe.
func RegisterProvider(p provider.Provider) {
	mu.Lock()
	defer mu.Unlock()
	providers[p.ID()] = p
}

// GetProvider retrieves a previously registered tool provider by its unique identifier
// (e.g., "github"). It is required for resolving package definitions before fetching.
func GetProvider(id string) (provider.Provider, error) {
	mu.RLock()
	defer mu.RUnlock()
	p, ok := providers[id]
	if !ok {
		return nil, fmt.Errorf("provider not found: %s", id)
	}
	return p, nil
}
