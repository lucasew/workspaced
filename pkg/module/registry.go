package module

import (
	"fmt"
	"strings"
	"sync"
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
		return nil, fmt.Errorf("unknown module provider %q", id)
	}
	return p, nil
}

func ResolveProviderAndRef(from string, moduleName string) (string, string, error) {
	f := strings.TrimSpace(from)
	if f == "" || f == "local" {
		return "local", moduleName, nil
	}
	parts := strings.SplitN(f, ":", 2)
	if len(parts) != 2 {
		return "", "", fmt.Errorf("invalid module from %q (expected provider:ref)", from)
	}
	provider := strings.TrimSpace(parts[0])
	ref := strings.TrimSpace(parts[1])
	if provider == "" || ref == "" {
		return "", "", fmt.Errorf("invalid module from %q (expected provider:ref)", from)
	}
	return provider, ref, nil
}
