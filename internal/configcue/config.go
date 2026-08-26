package configcue

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"cuelang.org/go/cue"
	"github.com/lucasew/workspaced/pkg/driver"
	envdriver "github.com/lucasew/workspaced/pkg/driver/env"
	"github.com/lucasew/workspaced/pkg/filespine"
	"github.com/lucasew/workspaced/pkg/taskgroup"
)

var (
	ErrKeyNotFound = errors.New("key not found in config")
	ErrKeyNotMap   = errors.New("config key is not a map")
)

type Config struct {
	raw    map[string]any
	cueVal cue.Value
}

// Cue returns the evaluated workspaced CUE value. Zero if the config was
// decoded from JSON only.
func (c *Config) Cue() cue.Value {
	if c == nil {
		return cue.Value{}
	}
	return c.cueVal
}

// FileMap is workspaced.file after unify.
func (c *Config) FileMap() (map[string]filespine.File, error) {
	return filespine.ParseAt(c.Cue(), "file")
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
			return nil, fmt.Errorf("%w: %q", ErrKeyNotMap, key)
		}
		next, ok := m[part]
		if !ok {
			return nil, fmt.Errorf("%w: %q", ErrKeyNotFound, key)
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

// ConcurrencyLimits reads the concurrency settings from config, falling back to defaults.
func (c *Config) ConcurrencyLimits() taskgroup.Limits {
	defaults := taskgroup.DefaultLimits()
	var raw struct {
		IO       *int `json:"io"`
		CPU      *int `json:"cpu"`
		Internet *int `json:"internet"`
	}
	if err := c.Decode("concurrency", &raw); err != nil {
		return defaults
	}
	if raw.IO != nil && *raw.IO > 0 {
		defaults.IO = *raw.IO
	}
	if raw.CPU != nil && *raw.CPU > 0 {
		defaults.CPU = *raw.CPU
	}
	if raw.Internet != nil && *raw.Internet > 0 {
		defaults.Internet = *raw.Internet
	}
	return defaults
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

func Load(ctx context.Context) (*Config, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return nil, err
	}
	return loadConfig(ctx, DiscoverOptions{Cwd: cwd})
}

func LoadHome(ctx context.Context) (*Config, error) {
	return loadConfig(ctx, DiscoverOptions{HomeMode: true})
}

func LoadForWorkspace(ctx context.Context, root string) (*Config, error) {
	root = strings.TrimSpace(root)
	if root == "" {
		return Load(ctx)
	}

	dotfilesRoot, err := envdriver.GetDotfilesRoot(ctx)
	if err == nil && filepath.Clean(dotfilesRoot) == filepath.Clean(root) {
		return LoadHome(ctx)
	}
	return loadConfig(ctx, DiscoverOptions{Cwd: root})
}

func LoadFiles(ctx context.Context, paths []string) (*Config, error) {
	if len(paths) == 0 {
		return Load(ctx)
	}
	configValue, err := buildWorkspacedValue(ctx, paths, nil, false)
	if err != nil {
		return nil, err
	}
	data, err := marshalWorkspacedValue(ctx, configValue, paths, nil)
	if err != nil {
		return nil, err
	}
	cfg, err := decodeConfig(data)
	if err != nil {
		return nil, err
	}
	cfg.cueVal = configValue
	return cfg, nil
}

func loadConfig(ctx context.Context, opts DiscoverOptions) (*Config, error) {
	result, err := Evaluate(ctx, opts)
	if err != nil {
		return nil, err
	}
	cfg, err := decodeConfig(result.JSON)
	if err != nil {
		return nil, err
	}
	cfg.cueVal = result.Value
	return cfg, nil
}

func decodeConfig(data []byte) (*Config, error) {
	raw := map[string]any{}
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, fmt.Errorf("decode exported cue config: %w", err)
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
