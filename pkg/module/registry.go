package module

import (
	"errors"
	"fmt"
	"strings"
	"sync"
)

var (
	ErrUnknownProvider  = errors.New("unknown module provider")
	ErrInvalidModuleRef = errors.New("invalid module from (expected provider:ref)")
)

var (
	providers   = map[string]Provider{}
	providersMu sync.RWMutex
)

func RegisterProvider(p Provider) {
	providersMu.Lock()
	defer providersMu.Unlock()
	providers[p.ID()] = p
}

func GetProvider(id string) (Provider, error) {
	providersMu.RLock()
	defer providersMu.RUnlock()
	p, ok := providers[id]
	if !ok {
		return nil, fmt.Errorf("%w: %q", ErrUnknownProvider, id)
	}
	return p, nil
}

func ResolveProviderAndRef(from string, moduleName string) (string, string, error) {
	f := strings.TrimSpace(from)
	if f == "" || f == "self" || f == "local" {
		// "local" kept as a compatibility alias for the pre-rename provider id.
		return "self", moduleName, nil
	}
	parts := strings.SplitN(f, ":", 2)
	if len(parts) != 2 {
		return "", "", fmt.Errorf("%w: %q", ErrInvalidModuleRef, from)
	}
	provider := strings.TrimSpace(parts[0])
	ref := strings.TrimSpace(parts[1])
	if provider == "" || ref == "" {
		return "", "", fmt.Errorf("%w: %q", ErrInvalidModuleRef, from)
	}
	return provider, ref, nil
}

var (
	coreModules   = map[string]CoreModule{}
	coreModulesMu sync.RWMutex
)

// RegisterCoreModule registers a module that will be available as core:<ref>.
func RegisterCoreModule(m CoreModule) {
	coreModulesMu.Lock()
	defer coreModulesMu.Unlock()
	coreModules[m.Ref()] = m
}

// GetCoreModule returns the core module registered for the given ref (the part after "core:").
func GetCoreModule(ref string) (CoreModule, bool) {
	coreModulesMu.RLock()
	defer coreModulesMu.RUnlock()
	m, ok := coreModules[ref]
	return m, ok
}
