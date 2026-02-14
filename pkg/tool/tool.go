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

func RegisterProvider(p provider.Provider) {
	mu.Lock()
	defer mu.Unlock()
	providers[p.ID()] = p
}

func GetProvider(id string) (provider.Provider, error) {
	mu.RLock()
	defer mu.RUnlock()
	p, ok := providers[id]
	if !ok {
		return nil, fmt.Errorf("provider not found: %s", id)
	}
	return p, nil
}
