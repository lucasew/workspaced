// Package tool manages installation and execution of external command-line tools
// (the things users reference as "uv", "github:cli/cli", "mise:node", etc.).
//
// It is built around a small set of registered tool backends (see pkg/tool/backend).
// The main entry points are NewManager, Ensure, and the lazy tool machinery.
package tool

import (
	"fmt"
	"sync"
	"workspaced/pkg/tool/backend"
)

var (
	mu       sync.RWMutex
	backends = make(map[string]backend.Backend)
)

// Register installs a backend under the given id.
// The id is what appears before the ':' in user specs ("id:ref@version").
// This is the canonical registration for the thin backend pattern.
func Register(id string, b backend.Backend) {
	mu.Lock()
	defer mu.Unlock()
	backends[id] = b
}

// Get retrieves a registered backend by its id.
func Get(id string) (backend.Backend, error) {
	mu.RLock()
	defer mu.RUnlock()
	b, ok := backends[id]
	if !ok {
		return nil, fmt.Errorf("tool backend not found: %s", id)
	}
	return b, nil
}

// --- Transitional shims ---
// These keep old call sites compiling while we migrate to the new Register/Get names
// and the Backend interface. They will be removed after migration.

func RegisterProvider(p backend.Backend) {
	// During transition the concrete backends no longer have ID(), so we cannot
	// recover the id here. Call sites in inits are being updated to use Register directly.
	// This shim is a no-op placeholder to avoid immediate compile errors in case
	// something still calls it; real registration now happens via Register(id, b).
	_ = p
}

func GetProvider(id string) (backend.Backend, error) {
	return Get(id)
}
