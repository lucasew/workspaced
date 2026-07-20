// Package tool manages installation and execution of external command-line tools
// (the things users reference as "uv", "github:cli/cli", "mise:node", etc.).
//
// It is built around a small set of registered tool backends (see internal/tool/backend).
// The main entry points are NewManager, Ensure, and the lazy tool machinery.
package tool

import (
	"errors"
	"fmt"
	"github.com/lucasew/workspaced/internal/tool/backend"
	"sync"
)

var (
	// ErrBackendNotFound is returned when a requested tool backend is not registered.
	ErrBackendNotFound = errors.New("tool backend not found")
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
		return nil, fmt.Errorf("%w: %s", ErrBackendNotFound, id)
	}
	return b, nil
}
