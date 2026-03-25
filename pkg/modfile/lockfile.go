package modfile

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"workspaced/pkg/config"
)

func IsLockableProvider(provider string) bool {
	switch strings.TrimSpace(provider) {
	case "self", "core":
		return false
	default:
		return true
	}
}

func BuildSourceLockEntries(modFile *ModFile) map[string]LockedSource {
	out := map[string]LockedSource{}
	if modFile == nil {
		return out
	}
	for name, src := range modFile.Sources {
		provider := strings.TrimSpace(src.Provider)
		if provider == "" {
			provider = strings.TrimSpace(name)
		}
		if !IsLockableProvider(provider) {
			continue
		}
		if p, ok := getSourceProvider(provider); ok {
			src = p.Normalize(src)
		}
		out[name] = LockedSource{
			Provider: provider,
			Path:     strings.TrimSpace(src.Path),
			Repo:     strings.TrimSpace(src.Repo),
			URL:      strings.TrimSpace(src.URL),
		}
	}
	return out
}

func BuildLockEntries(cfg *config.GlobalConfig, modFile *ModFile, modulesBaseDir string) (map[string]LockedModule, error) {
	out := map[string]LockedModule{}
	if cfg == nil {
		return out, nil
	}

	moduleNames := make([]string, 0, len(cfg.Modules))
	for name := range cfg.Modules {
		moduleNames = append(moduleNames, name)
	}
	sort.Strings(moduleNames)

	for _, modName := range moduleNames {
		modCfgRaw := cfg.Modules[modName]
		if modCfgRaw == nil {
			continue
		}
		modCfg, ok := modCfgRaw.(map[string]any)
		if !ok {
			return nil, fmt.Errorf("invalid config for module %q: expected map, got %T", modName, modCfgRaw)
		}
		enabled, _ := modCfg["enable"].(bool)
		if !enabled {
			continue
		}

		resolved, err := ResolveModuleFromConfig(cfg, modName, modCfg, modulesBaseDir, nil)
		if err != nil {
			return nil, fmt.Errorf("module %q: %w", modName, err)
		}
		if !IsLockableProvider(resolved.Provider) {
			continue
		}

		out[modName] = LockedModule{
			Source:  resolved.Provider + ":" + resolved.Ref,
			Version: resolved.Version,
		}
	}

	return out, nil
}

func WriteSumFile(path string, sum *SumFile) error {
	if sum == nil {
		sum = &SumFile{Sources: map[string]LockedSource{}, Modules: map[string]LockedModule{}}
	}
	if sum.Sources == nil {
		sum.Sources = map[string]LockedSource{}
	}
	if sum.Modules == nil {
		sum.Modules = map[string]LockedModule{}
	}

	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}

	tmpPath := path + ".tmp"
	f, err := os.Create(tmpPath)
	if err != nil {
		return err
	}

	enc := json.NewEncoder(f)
	enc.SetIndent("", "  ")
	if err := enc.Encode(sum); err != nil {
		_ = f.Close()
		return err
	}

	if err := f.Close(); err != nil {
		return err
	}
	return os.Rename(tmpPath, path)
}
