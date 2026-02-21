package modfile

import (
	"fmt"
	"os"
	"strings"

	"github.com/BurntSushi/toml"
)

type LockedModule struct {
	Source  string `toml:"source"`
	Version string `toml:"version"`
}

type LockedSource struct {
	Provider string `toml:"provider"`
	Path     string `toml:"path"`
	Repo     string `toml:"repo"`
	URL      string `toml:"url"`
}

type SumFile struct {
	Sources map[string]LockedSource `toml:"sources"`
	Modules map[string]LockedModule `toml:"modules"`
}

func LoadSumFile(path string) (*SumFile, error) {
	out := &SumFile{
		Sources: map[string]LockedSource{},
		Modules: map[string]LockedModule{},
	}
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return out, nil
	}
	if _, err := toml.DecodeFile(path, out); err != nil {
		return nil, fmt.Errorf("failed to parse %s: %w", path, err)
	}
	if out.Sources == nil {
		out.Sources = map[string]LockedSource{}
	}
	if out.Modules == nil {
		out.Modules = map[string]LockedModule{}
	}
	for name, lock := range out.Sources {
		lock.Provider = strings.TrimSpace(lock.Provider)
		lock.Path = strings.TrimSpace(lock.Path)
		lock.Repo = strings.TrimSpace(lock.Repo)
		lock.URL = strings.TrimSpace(lock.URL)
		if lock.Provider == "" {
			return nil, fmt.Errorf("invalid lock entry for source %q: provider is required", name)
		}
		out.Sources[name] = lock
	}
	for mod, lock := range out.Modules {
		lock.Source = strings.TrimSpace(lock.Source)
		lock.Version = strings.TrimSpace(lock.Version)
		if lock.Source == "" {
			return nil, fmt.Errorf("invalid lock entry for module %q: source is required", mod)
		}
		out.Modules[mod] = lock
	}
	return out, nil
}
