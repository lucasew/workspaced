package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"workspaced/pkg/configcue"
	"workspaced/pkg/driver"
	"workspaced/pkg/env"
)

type GlobalConfig struct {
	Workspaces map[string]int            `json:"workspaces"`
	Desktop    DesktopConfig             `json:"desktop"`
	Screenshot ScreenshotConfig          `json:"screenshot"`
	Hosts      map[string]HostConfig     `json:"hosts"`
	Backup     BackupConfig              `json:"backup"`
	QuickSync  QuickSyncConfig           `json:"quicksync"`
	Browser    BrowserConfig             `json:"browser"`
	LazyTools  map[string]LazyToolConfig `json:"lazy_tools"`
	Inputs     map[string]InputConfig    `json:"inputs"`
	Modules    map[string]any            `json:"modules"`
	Drivers    map[string]map[string]int `json:"drivers"`

	raw map[string]any
}

type InputConfig struct {
	From    string `json:"from"`
	Version string `json:"version"`
}

type DesktopConfig struct {
	DarkMode  bool            `json:"dark_mode"`
	Wallpaper WallpaperConfig `json:"wallpaper"`
}

type LazyToolConfig struct {
	Version string   `json:"version"`
	Ref     string   `json:"ref"`
	Pkg     string   `json:"pkg"`
	Global  bool     `json:"global"`
	Alias   string   `json:"alias"`
	Bins    []string `json:"bins"`
}

type HostConfig struct {
	IPs  []string `json:"ips"`
	MAC  string   `json:"mac"`
	Port int      `json:"port"`
	User string   `json:"user"`
}

type BrowserConfig struct {
	Default string `json:"default"`
	Engine  string `json:"webapp"`
}

type WallpaperConfig struct {
	Dir     string `json:"dir"`
	Default string `json:"default"`
}

type ScreenshotConfig struct {
	Dir string `json:"dir"`
}

type BackupConfig struct {
	RsyncnetUser string `json:"rsyncnet_user"`
	RemotePath   string `json:"remote_path"`
}

type QuickSyncConfig struct {
	RepoDir    string `json:"repo_dir"`
	RemotePath string `json:"remote_path"`
}

type PaletteConfig struct {
	Base00 string `json:"base00"`
	Base01 string `json:"base01"`
	Base02 string `json:"base02"`
	Base03 string `json:"base03"`
	Base04 string `json:"base04"`
	Base05 string `json:"base05"`
	Base06 string `json:"base06"`
	Base07 string `json:"base07"`
	Base08 string `json:"base08"`
	Base09 string `json:"base09"`
	Base0A string `json:"base0A"`
	Base0B string `json:"base0B"`
	Base0C string `json:"base0C"`
	Base0D string `json:"base0D"`
	Base0E string `json:"base0E"`
	Base0F string `json:"base0F"`
	Base10 string `json:"base10,omitempty"`
	Base11 string `json:"base11,omitempty"`
	Base12 string `json:"base12,omitempty"`
	Base13 string `json:"base13,omitempty"`
	Base14 string `json:"base14,omitempty"`
	Base15 string `json:"base15,omitempty"`
	Base16 string `json:"base16,omitempty"`
	Base17 string `json:"base17,omitempty"`
}

func (c *GlobalConfig) Raw() map[string]any {
	if c == nil || c.raw == nil {
		return map[string]any{}
	}
	return c.raw
}

func (c *GlobalConfig) UnmarshalKey(key string, val any) error {
	current := any(c.Raw())
	for _, part := range strings.Split(key, ".") {
		m, ok := current.(map[string]any)
		if !ok {
			return fmt.Errorf("key %q not found or not a map", key)
		}
		next, ok := m[part]
		if !ok {
			return fmt.Errorf("key %q not found in config", key)
		}
		current = next
	}
	data, err := json.Marshal(current)
	if err != nil {
		return err
	}
	return json.Unmarshal(data, val)
}

func (c *GlobalConfig) Module(name string, target any) error {
	if err := c.UnmarshalKey("modules."+name+".config", target); err == nil {
		return nil
	}
	return c.UnmarshalKey("modules."+name, target)
}

func Load() (*GlobalConfig, error) {
	cwd, _ := os.Getwd()
	return load(configcue.DiscoverOptions{Cwd: cwd})
}

func LoadHome() (*GlobalConfig, error) {
	return load(configcue.DiscoverOptions{HomeMode: true})
}

func LoadForWorkspace(root string) (*GlobalConfig, error) {
	root = strings.TrimSpace(root)
	if root == "" {
		return Load()
	}

	dotfilesRoot, err := env.GetDotfilesRoot()
	if err == nil && filepath.Clean(dotfilesRoot) == filepath.Clean(root) {
		return LoadHome()
	}

	return load(configcue.DiscoverOptions{Cwd: root})
}

func LoadConfigForWorkspace(root string) (*GlobalConfig, error) {
	return LoadForWorkspace(root)
}

func LoadConfigHome() (*GlobalConfig, error) {
	return LoadHome()
}

func LoadFiles(paths []string) (*GlobalConfig, error) {
	if len(paths) == 0 {
		return Load()
	}
	b, err := configcue.ExportJSONFromPaths(paths)
	if err != nil {
		return nil, err
	}
	return decodeConfig(b)
}

func load(opts configcue.DiscoverOptions) (*GlobalConfig, error) {
	result, err := configcue.Evaluate(opts)
	if err != nil {
		return nil, err
	}
	return decodeConfig(result.JSON)
}

func decodeConfig(data []byte) (*GlobalConfig, error) {
	raw := map[string]any{}
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, fmt.Errorf("failed to decode exported cue config: %w", err)
	}
	normalizeModuleEntries(raw)

	cfg := &GlobalConfig{}
	typed, err := json.Marshal(raw)
	if err != nil {
		return nil, fmt.Errorf("failed to remarshal config: %w", err)
	}
	if err := json.Unmarshal(typed, cfg); err != nil {
		return nil, fmt.Errorf("failed to decode typed config: %w", err)
	}

	cfg.raw = raw
	cfg.Desktop.Wallpaper.Dir = env.ExpandPath(cfg.Desktop.Wallpaper.Dir)
	cfg.Desktop.Wallpaper.Default = env.ExpandPath(cfg.Desktop.Wallpaper.Default)
	cfg.Screenshot.Dir = env.ExpandPath(cfg.Screenshot.Dir)
	cfg.QuickSync.RepoDir = env.ExpandPath(cfg.QuickSync.RepoDir)
	if err := driver.SetWeights(cfg.Drivers); err != nil {
		return nil, err
	}
	return cfg, nil
}

func normalizeModuleEntries(raw map[string]any) {
	modsRaw, ok := raw["modules"]
	if !ok {
		return
	}
	mods, ok := modsRaw.(map[string]any)
	if !ok {
		return
	}
	for name, entryRaw := range mods {
		entry, ok := entryRaw.(map[string]any)
		if !ok {
			continue
		}
		configRaw, hasConfig := entry["config"]
		if !hasConfig {
			continue
		}
		cfgMap, ok := configRaw.(map[string]any)
		if !ok {
			continue
		}
		flattened := make(map[string]any, len(entry)+len(cfgMap))
		for k, v := range cfgMap {
			flattened[k] = v
		}
		for k, v := range entry {
			if k == "config" {
				continue
			}
			flattened[k] = v
		}
		mods[name] = flattened
	}
}
