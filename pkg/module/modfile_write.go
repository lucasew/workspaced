package module

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
)

func WriteModFile(path string, mod *ModFile) error {
	if mod == nil {
		mod = &ModFile{
			Sources: map[string]SourceConfig{},
			Modules: map[string]string{},
		}
	}
	if mod.Sources == nil {
		mod.Sources = map[string]SourceConfig{}
	}
	if mod.Modules == nil {
		mod.Modules = map[string]string{}
	}

	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}

	tmpPath := path + ".tmp"
	f, err := os.Create(tmpPath)
	if err != nil {
		return err
	}

	sourceNames := make([]string, 0, len(mod.Sources))
	for name := range mod.Sources {
		sourceNames = append(sourceNames, name)
	}
	sort.Strings(sourceNames)

	for _, name := range sourceNames {
		src := mod.Sources[name]
		if _, err := fmt.Fprintf(f, "[sources.%s]\n", name); err != nil {
			_ = f.Close()
			return err
		}
		if strings.TrimSpace(src.Provider) != "" {
			if _, err := fmt.Fprintf(f, "provider = %s\n", strconv.Quote(src.Provider)); err != nil {
				_ = f.Close()
				return err
			}
		}
		if strings.TrimSpace(src.Path) != "" {
			if _, err := fmt.Fprintf(f, "path = %s\n", strconv.Quote(src.Path)); err != nil {
				_ = f.Close()
				return err
			}
		}
		if strings.TrimSpace(src.Repo) != "" {
			if _, err := fmt.Fprintf(f, "repo = %s\n", strconv.Quote(src.Repo)); err != nil {
				_ = f.Close()
				return err
			}
		}
		if strings.TrimSpace(src.URL) != "" {
			if _, err := fmt.Fprintf(f, "url = %s\n", strconv.Quote(src.URL)); err != nil {
				_ = f.Close()
				return err
			}
		}
		if _, err := fmt.Fprintf(f, "\n"); err != nil {
			_ = f.Close()
			return err
		}
	}

	moduleNames := make([]string, 0, len(mod.Modules))
	for name := range mod.Modules {
		moduleNames = append(moduleNames, name)
	}
	sort.Strings(moduleNames)

	if _, err := fmt.Fprintf(f, "[modules]\n"); err != nil {
		_ = f.Close()
		return err
	}
	for _, name := range moduleNames {
		spec := strings.TrimSpace(mod.Modules[name])
		if spec == "" {
			_ = f.Close()
			return fmt.Errorf("invalid module entry for %q: empty spec", name)
		}
		if _, err := fmt.Fprintf(f, "%s = %s\n", name, strconv.Quote(spec)); err != nil {
			_ = f.Close()
			return err
		}
	}

	if err := f.Close(); err != nil {
		return err
	}
	return os.Rename(tmpPath, path)
}
