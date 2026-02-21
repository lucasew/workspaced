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

type SumFile struct {
	Modules map[string]LockedModule `toml:"modules"`
}

func LoadSumFile(path string) (*SumFile, error) {
	out := &SumFile{
		Modules: map[string]LockedModule{},
	}
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return out, nil
	}
	if _, err := toml.DecodeFile(path, out); err != nil {
		return nil, fmt.Errorf("failed to parse %s: %w", path, err)
	}
	if out.Modules == nil {
		out.Modules = map[string]LockedModule{}
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
