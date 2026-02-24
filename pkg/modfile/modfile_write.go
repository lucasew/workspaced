package modfile

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
		}
	}
	if mod.Sources == nil {
		mod.Sources = map[string]SourceConfig{}
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

	if len(sourceNames) > 0 {
		if _, err := fmt.Fprintf(f, "[sources]\n"); err != nil {
			_ = f.Close()
			return err
		}
		for _, name := range sourceNames {
			src := mod.Sources[name]
			spec := formatSourceSpec(name, src)
			if _, err := fmt.Fprintf(f, "%s = %s\n", name, strconv.Quote(spec)); err != nil {
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

func formatSourceSpec(alias string, src SourceConfig) string {
	provider := strings.TrimSpace(src.Provider)
	if provider == "" {
		provider = strings.TrimSpace(alias)
	}

	target := ""
	switch provider {
	case "github":
		target = strings.TrimSpace(src.Repo)
		if target == "" {
			target = strings.TrimSpace(src.Path)
		}
	case "local":
		target = strings.TrimSpace(src.Path)
	default:
		target = strings.TrimSpace(src.Path)
		if target == "" {
			target = strings.TrimSpace(src.Repo)
		}
		if target == "" {
			target = strings.TrimSpace(src.URL)
		}
	}

	spec := provider + ":" + target
	if strings.TrimSpace(src.Ref) != "" {
		spec += "@" + strings.TrimSpace(src.Ref)
	}
	return spec
}
