package tool

import (
	"fmt"
	"strings"
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

// Parse "github:denoland/deno@1.40.0" -> ("github", "denoland/deno", "1.40.0")
func ParseToolSpec(spec string) (providerID, pkg, version string, err error) {
	parts := strings.SplitN(spec, ":", 2)
	if len(parts) != 2 {
		return "", "", "", fmt.Errorf("invalid tool spec: %s (expected provider:pkg@version)", spec)
	}
	providerID = parts[0]
	rest := parts[1]

	parts = strings.SplitN(rest, "@", 2)
	pkg = parts[0]
	if len(parts) == 2 {
		version = parts[1]
	} else {
		version = "latest"
	}

	return providerID, pkg, version, nil
}

// SpecToDir normalizes spec to directory name: "github:denoland/deno" -> "github-denoland-deno"
func SpecToDir(providerID, pkgSpec string) string {
	s := fmt.Sprintf("%s-%s", providerID, pkgSpec)
	s = strings.ReplaceAll(s, "/", "-")
	s = strings.ReplaceAll(s, ":", "-")
	return s
}
