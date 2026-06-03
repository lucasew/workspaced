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

// Register installs a provider handler under the given id.
// The id is what appears before the ':' in user specs ("id:ref@version").
// This is the canonical registration for the thin handler pattern.
func Register(id string, p provider.Provider) {
	mu.Lock()
	defer mu.Unlock()
	providers[id] = p
}

// Get retrieves a registered provider handler by its id.
func Get(id string) (provider.Provider, error) {
	mu.RLock()
	defer mu.RUnlock()
	p, ok := providers[id]
	if !ok {
		return nil, fmt.Errorf("provider not found: %s", id)
	}
	return p, nil
}

// --- Transitional shims ---
// These keep old call sites compiling while we migrate to the new Register/Get names
// and the thin Provider interface. They will be removed after migration.

func RegisterProvider(p provider.Provider) {
	// During transition the concrete providers no longer have ID(), so we cannot
	// recover the id here. Call sites in inits are being updated to use Register directly.
	// This shim is a no-op placeholder to avoid immediate compile errors in case
	// something still calls it; real registration now happens via Register(id, p).
	_ = p
}

func GetProvider(id string) (provider.Provider, error) {
	return Get(id)
}
