package configcue

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"workspaced/pkg/driver"
	"workspaced/pkg/env"
)

type Config struct {
	raw map[string]any
}

type Input struct {
	From    string `json:"from"`
	Version string `json:"version"`
}

type ModuleEntry struct {
	Enable  bool           `json:"enable"`
	Input   string         `json:"input"`
	Path    string         `json:"path"`
	From    string         `json:"from"`
	Version string         `json:"version"`
	Config  map[string]any `json:"config"`
}

func (c *Config) Raw() map[string]any {
	if c == nil || c.raw == nil {
		return map[string]any{}
	}
	return c.raw
}

func (c *Config) Lookup(key string) (any, error) {
	if strings.TrimSpace(key) == "" {
		return c.Raw(), nil
	}

	current := any(c.Raw())
	for _, part := range strings.Split(key, ".") {
		m, ok := current.(map[string]any)
		if !ok {
			return nil, fmt.Errorf("key %q not found or not a map", key)
		}
		next, ok := m[part]
		if !ok {
			return nil, fmt.Errorf("key %q not found in config", key)
		}
		current = next
	}
	return current, nil
}

func (c *Config) Decode(key string, val any) error {
	current, err := c.Lookup(key)
	if err != nil {
		return err
	}
	data, err := json.Marshal(current)
	if err != nil {
		return err
	}
	return json.Unmarshal(data, val)
}

func (c *Config) ModuleConfig(name string, target any) error {
	if err := c.Decode("modules."+name+".config", target); err == nil {
		return nil
	}
	return c.Decode("modules."+name, target)
}

func (c *Config) Inputs() (map[string]Input, error) {
	inputs := map[string]Input{}
	if err := c.Decode("inputs", &inputs); err != nil {
		return nil, err
	}
	return inputs, nil
}

func (c *Config) Modules() (map[string]ModuleEntry, error) {
	modules := map[string]ModuleEntry{}
	if err := c.Decode("modules", &modules); err != nil {
		return nil, err
	}
	for name, entry := range modules {
		if entry.Config == nil {
			entry.Config = map[string]any{}
		}
		modules[name] = entry
	}
	return modules, nil
}

func (c *Config) ModuleEntry(name string) (ModuleEntry, error) {
	var entry ModuleEntry
	if err := c.Decode("modules."+name, &entry); err != nil {
		return ModuleEntry{}, err
	}
	if entry.Config == nil {
		entry.Config = map[string]any{}
	}
	return entry, nil
}

func Load() (*Config, error) {
	cwd, _ := os.Getwd()
	return loadConfig(DiscoverOptions{Cwd: cwd})
}

func LoadHome() (*Config, error) {
	return loadConfig(DiscoverOptions{HomeMode: true})
}

func LoadForWorkspace(root string) (*Config, error) {
	root = strings.TrimSpace(root)
	if root == "" {
		return Load()
	}

	dotfilesRoot, err := env.GetDotfilesRoot()
	if err == nil && filepath.Clean(dotfilesRoot) == filepath.Clean(root) {
		return LoadHome()
	}
	return loadConfig(DiscoverOptions{Cwd: root})
}

func LoadFiles(paths []string) (*Config, error) {
	if len(paths) == 0 {
		return Load()
	}
	data, err := ExportJSONFromPaths(paths)
	if err != nil {
		return nil, err
	}
	return decodeConfig(data)
}

func loadConfig(opts DiscoverOptions) (*Config, error) {
	result, err := Evaluate(opts)
	if err != nil {
		return nil, err
	}
	return decodeConfig(result.JSON)
}

func decodeConfig(data []byte) (*Config, error) {
	raw := map[string]any{}
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, fmt.Errorf("failed to decode exported cue config: %w", err)
	}
	if driversRaw, ok := raw["drivers"]; ok {
		typed := map[string]map[string]int{}
		if data, err := json.Marshal(driversRaw); err == nil {
			if err := json.Unmarshal(data, &typed); err == nil {
				if err := driver.SetWeights(typed); err != nil {
					return nil, err
				}
			}
		}
	}
	return &Config{raw: raw}, nil
}
