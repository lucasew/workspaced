package modfile

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
)

// TryResolveSourceRefToPath tries to resolve "alias:path" using modfile sources.
// It returns (resolvedPath, true, nil) when the spec is a resolvable source ref.
// It returns (spec, false, nil) when the input should be treated as a regular path.
func (m *ModFile) TryResolveSourceRefToPath(ctx context.Context, spec string, modulesBaseDir string) (string, bool, error) {
	parts := strings.SplitN(strings.TrimSpace(spec), ":", 2)
	if len(parts) != 2 || strings.TrimSpace(parts[0]) == "" || strings.TrimSpace(parts[1]) == "" {
		return spec, false, nil
	}
	alias := strings.TrimSpace(parts[0])
	rel := strings.Trim(strings.TrimSpace(parts[1]), "/")
	if rel == "" {
		return spec, false, nil
	}

	src, ok := m.Sources[alias]
	if !ok {
		// Support direct provider form for built-ins like local:path
		src = SourceConfig{Provider: alias}
	}
	providerID := strings.TrimSpace(src.Provider)
	if providerID == "" {
		providerID = alias
	}
	provider, ok := getSourceProvider(providerID)
	if ok {
		normalized := provider.Normalize(src)
		out, err := provider.ResolvePath(ctx, alias, normalized, rel, modulesBaseDir)
		if err != nil {
			return "", false, err
		}
		return out, true, nil
	}

	// "self" (and local) have no SourceProvider registered, but are valid for
	// input refs ("alias:rel"). "self" is always present as an input (see
	// ModFileFromConfig), so you can directly write "self:subdir" (or
	// "self:.") in core:place items etc. Mirror the base path logic.
	if providerID == "self" || providerID == "local" {
		base := filepath.Dir(modulesBaseDir)
		customBase := strings.TrimSpace(src.Path)
		if customBase != "" {
			if filepath.IsAbs(customBase) {
				base = customBase
			} else {
				base = filepath.Join(base, customBase)
			}
		} else if providerID == "local" {
			base = modulesBaseDir
		}
		return filepath.Join(base, rel), true, nil
	}

	if _, exists := m.Sources[alias]; !exists {
		return spec, false, nil
	}
	return "", false, fmt.Errorf("source alias %q provider %q is not supported for input refs", alias, providerID)
}
