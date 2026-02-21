package module

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"workspaced/pkg/config"
)

func IsLockableProvider(provider string) bool {
	switch strings.TrimSpace(provider) {
	case "local", "core":
		return false
	default:
		return true
	}
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

		from, _ := modCfg["from"].(string)
		resolved, err := modFile.ResolveModuleSource(modName, from, modulesBaseDir, nil)
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
		sum = &SumFile{Modules: map[string]LockedModule{}}
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

	names := make([]string, 0, len(sum.Modules))
	for name := range sum.Modules {
		names = append(names, name)
	}
	sort.Strings(names)

	for i, name := range names {
		entry := sum.Modules[name]
		if strings.TrimSpace(entry.Source) == "" {
			_ = f.Close()
			return fmt.Errorf("invalid lock entry for module %q: source is required", name)
		}
		if i > 0 {
			if _, err := fmt.Fprintf(f, "\n"); err != nil {
				_ = f.Close()
				return err
			}
		}
		if _, err := fmt.Fprintf(f, "[modules.%s]\n", name); err != nil {
			_ = f.Close()
			return err
		}
		if _, err := fmt.Fprintf(f, "source = %s\n", strconv.Quote(entry.Source)); err != nil {
			_ = f.Close()
			return err
		}
		if strings.TrimSpace(entry.Version) != "" {
			if _, err := fmt.Fprintf(f, "version = %s\n", strconv.Quote(entry.Version)); err != nil {
				_ = f.Close()
				return err
			}
		}
	}

	if err := f.Close(); err != nil {
		return err
	}
	return os.Rename(tmpPath, path)
}
