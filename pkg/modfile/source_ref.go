package modfile

import (
	"fmt"
	"path/filepath"
	"strings"
)

// TryResolveSourceRefToPath tries to resolve "alias:path" using modfile sources.
// It returns (resolvedPath, true, nil) when the spec is a resolvable source ref.
// It returns (spec, false, nil) when the input should be treated as a regular path.
func (m *ModFile) TryResolveSourceRefToPath(spec string, modulesBaseDir string) (string, bool, error) {
	parts := strings.SplitN(strings.TrimSpace(spec), ":", 2)
	if len(parts) != 2 || strings.TrimSpace(parts[0]) == "" || strings.TrimSpace(parts[1]) == "" {
		return spec, false, nil
	}
	alias := strings.TrimSpace(parts[0])
	rel := strings.Trim(strings.TrimSpace(parts[1]), "/")
	if rel == "" {
		return spec, false, nil
	}

	resolveBase := func(base string) string {
		if strings.TrimSpace(base) == "" {
			return modulesBaseDir
		}
		if filepath.IsAbs(base) {
			return base
		}
		return filepath.Join(filepath.Dir(modulesBaseDir), base)
	}

	if alias == "local" {
		return filepath.Join(resolveBase(""), rel), true, nil
	}

	src, ok := m.Sources[alias]
	if !ok {
		return spec, false, nil
	}
	provider := strings.TrimSpace(src.Provider)
	if provider == "" {
		provider = alias
	}
	if provider != "local" {
		return "", false, fmt.Errorf("source alias %q provider %q is not supported for input refs (use local)", alias, provider)
	}

	return filepath.Join(resolveBase(src.Path), rel), true, nil
}
