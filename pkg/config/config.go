package config

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"workspaced/pkg/configcue"
	"workspaced/pkg/driver"
	"workspaced/pkg/env"
	"workspaced/pkg/modulecue"
)

type Config struct {
	*GlobalConfig
	raw map[string]any
}

func (c *Config) Raw() map[string]any {
	if c == nil || c.raw == nil {
		return map[string]any{}
	}
	return c.raw
}

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

func (c *Config) Module(name string, target any) error {
	return c.UnmarshalKey("modules."+name, target)
}

func (c *Config) UnmarshalKey(key string, val any) error {
	parts := strings.Split(key, ".")
	var current any = c.raw
	for _, part := range parts {
		if mRaw, ok := current.(map[string]any); ok {
			v, ok := mRaw[part]
			if !ok {
				return fmt.Errorf("key %q not found in config", key)
			}
			current = v
		} else if mRaw, ok := current.(map[string]any); ok {
			v, ok := mRaw[part]
			if !ok {
				return fmt.Errorf("key %q not found in config", key)
			}
			current = v
		} else {
			return fmt.Errorf("key %q not found or not a map", key)
		}
	}
	if current == nil {
		return fmt.Errorf("value for key %q is nil", key)
	}
	data, err := json.Marshal(current)
	if err != nil {
		return err
	}
	return json.Unmarshal(data, val)
}

func Load() (*Config, error) {
	return loadFromCUE()
}

func LoadHome() (*Config, error) {
	return loadFromCUEWithOptions(configcue.DiscoverOptions{HomeMode: true})
}

func LoadForWorkspace(root string) (*Config, error) {
	root = strings.TrimSpace(root)
	if root == "" {
		return loadFromCUE()
	}

	dotfilesRoot, err := env.GetDotfilesRoot()
	if err == nil && filepath.Clean(dotfilesRoot) == filepath.Clean(root) {
		return LoadHome()
	}

	return loadFromCUEWithOptions(configcue.DiscoverOptions{Cwd: root})
}

func LoadFiles(paths []string) (*GlobalConfig, error) {
	if len(paths) == 0 {
		cfg, err := Load()
		if err != nil {
			return nil, err
		}
		return cfg.GlobalConfig, nil
	}
	gCfg, err := LoadConfigBase()
	if err != nil {
		return nil, err
	}
	rawMerged, err := structToMap(gCfg)
	if err != nil {
		return nil, fmt.Errorf("failed to convert default config to map: %w", err)
	}
	exported, err := configcue.ExportJSONFromPaths(paths)
	if err != nil {
		return nil, err
	}
	var currentRaw map[string]any
	if err := json.Unmarshal(exported, &currentRaw); err != nil {
		return nil, fmt.Errorf("failed to decode exported cue config: %w", err)
	}
	normalizeModuleEntries(currentRaw)
	enabledModules := enabledModulesFromRaw(currentRaw)
	moduleMeta, defaultsRaw, err := loadEnabledModuleDefinitions(enabledModules)
	if err != nil {
		return nil, err
	}
	if err := validateDependencies(enabledModules, moduleMeta); err != nil {
		return nil, err
	}
	if err := MergeStrict(rawMerged, defaultsRaw, true); err != nil {
		return nil, fmt.Errorf("failed to merge defaults into config: %w", err)
	}
	if err := MergeStrict(rawMerged, currentRaw, true); err != nil {
		return nil, err
	}
	cfg, err := finalizeConfig(rawMerged)
	if err != nil {
		return nil, err
	}
	return cfg.GlobalConfig, nil
}

func loadFromCUE() (*Config, error) {
	cwd, _ := os.Getwd()
	return loadFromCUEWithOptions(configcue.DiscoverOptions{Cwd: cwd})
}

func loadFromCUEWithOptions(opts configcue.DiscoverOptions) (*Config, error) {
	gCfg, err := LoadConfigBase()
	if err != nil {
		return nil, err
	}

	rawMerged, err := structToMap(gCfg)
	if err != nil {
		return nil, fmt.Errorf("failed to convert default config to map: %w", err)
	}

	evaluated, err := configcue.Evaluate(opts)
	if err != nil {
		return nil, err
	}

	var currentRaw map[string]any
	if err := json.Unmarshal(evaluated.JSON, &currentRaw); err != nil {
		return nil, fmt.Errorf("failed to decode exported cue config: %w", err)
	}
	normalizeModuleEntries(currentRaw)

	enabledModules := enabledModulesFromRaw(currentRaw)
	moduleMeta, defaultsRaw, err := loadEnabledModuleDefinitions(enabledModules)
	if err != nil {
		return nil, err
	}
	if err := validateDependencies(enabledModules, moduleMeta); err != nil {
		return nil, err
	}
	if err := MergeStrict(rawMerged, defaultsRaw, true); err != nil {
		return nil, fmt.Errorf("failed to merge defaults into config: %w", err)
	}
	if err := MergeStrict(rawMerged, currentRaw, true); err != nil {
		return nil, err
	}

	return finalizeConfig(rawMerged)
}

func LoadConfigForWorkspace(root string) (*GlobalConfig, error) {
	cfg, err := LoadForWorkspace(root)
	if err != nil {
		return nil, err
	}
	return cfg.GlobalConfig, nil
}

func LoadConfigHome() (*GlobalConfig, error) {
	cfg, err := LoadHome()
	if err != nil {
		return nil, err
	}
	return cfg.GlobalConfig, nil
}

func LoadConfigBase() (*GlobalConfig, error) {
	home, _ := os.UserHomeDir()
	dotfiles, _ := env.GetDotfilesRoot()
	return &GlobalConfig{
		Workspaces: map[string]int{"www": 1, "meet": 2},
		Desktop:    DesktopConfig{Wallpaper: WallpaperConfig{Dir: filepath.Join(dotfiles, "assets/wallpapers")}},
		Screenshot: ScreenshotConfig{Dir: filepath.Join(home, "Pictures/Screenshots")},
		Backup:     BackupConfig{RsyncnetUser: "de3163@de3163.rsync.net", RemotePath: "backup/lucasew"},
		QuickSync:  QuickSyncConfig{RepoDir: filepath.Join(home, ".personal"), RemotePath: "/data2/home/de3163/git-personal"},
		Hosts:      make(map[string]HostConfig),
		Browser:    BrowserConfig{Default: "zen", Engine: "brave"},
		LazyTools:  make(map[string]LazyToolConfig),
		Inputs:     make(map[string]InputConfig),
		Modules:    make(map[string]any),
		Drivers:    make(map[string]map[string]int),
	}, nil
}

func structToMap(s any) (map[string]any, error) {
	data, err := json.Marshal(s)
	if err != nil {
		return nil, err
	}
	var res map[string]any
	if err := json.Unmarshal(data, &res); err != nil {
		return nil, err
	}
	return res, nil
}

func enabledModulesFromRaw(raw map[string]any) map[string]bool {
	enabledModules := make(map[string]bool)
	modsRaw, ok := raw["modules"]
	if !ok {
		return enabledModules
	}
	modsVal := reflect.ValueOf(modsRaw)
	if modsVal.Kind() != reflect.Map {
		return enabledModules
	}
	for _, modKey := range modsVal.MapKeys() {
		mVal := modsVal.MapIndex(modKey)
		if mVal.Kind() == reflect.Interface {
			mVal = mVal.Elem()
		}
		if mVal.Kind() != reflect.Map {
			continue
		}
		eVal := mVal.MapIndex(reflect.ValueOf("enable"))
		if !eVal.IsValid() {
			continue
		}
		if eVal.Kind() == reflect.Interface {
			eVal = eVal.Elem()
		}
		if eVal.Kind() == reflect.Bool && eVal.Bool() {
			enabledModules[modKey.String()] = true
		}
	}
	return enabledModules
}

func loadEnabledModuleDefinitions(enabledModules map[string]bool) (map[string]ModuleMetadata, map[string]any, error) {
	dotfiles, _ := env.GetDotfilesRoot()
	modulesDir := filepath.Join(dotfiles, "modules")
	moduleMeta := make(map[string]ModuleMetadata)
	defaultsRaw := make(map[string]any)

	entries, err := os.ReadDir(modulesDir)
	if err != nil {
		if os.IsNotExist(err) {
			return moduleMeta, defaultsRaw, nil
		}
		return nil, nil, err
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		name := entry.Name()
		if !enabledModules[name] {
			continue
		}
		modPath := filepath.Join(modulesDir, name)
		if modulecue.Exists(modPath) {
			def, err := modulecue.Load(modPath)
			if err != nil {
				return nil, nil, err
			}
			moduleMeta[name] = ModuleMetadata{
				Requires:   def.Meta.Requires,
				Recommends: def.Meta.Recommends,
			}
			wrapped := map[string]any{"modules": map[string]any{name: def.Config}}
			if err := MergeStrict(defaultsRaw, wrapped, false); err != nil {
				return nil, nil, fmt.Errorf("failed to merge cue defaults for module %s: %w", name, err)
			}
			if len(def.Drivers) > 0 {
				wrappedDrivers := map[string]any{"drivers": any(def.Drivers)}
				if err := MergeStrict(defaultsRaw, wrappedDrivers, false); err != nil {
					return nil, nil, fmt.Errorf("failed to merge cue driver defaults for module %s: %w", name, err)
				}
			}
			continue
		}
		return nil, nil, fmt.Errorf("module %q is missing module.cue", name)
	}

	return moduleMeta, defaultsRaw, nil
}

func finalizeConfig(rawMerged map[string]any) (*Config, error) {
	finalGCfg := &GlobalConfig{}
	data, err := json.Marshal(rawMerged)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal merged config: %w", err)
	}
	if err := json.Unmarshal(data, finalGCfg); err != nil {
		return nil, fmt.Errorf("failed to unmarshal merged config: %w", err)
	}
	finalGCfg.Desktop.Wallpaper.Dir = env.ExpandPath(finalGCfg.Desktop.Wallpaper.Dir)
	finalGCfg.Desktop.Wallpaper.Default = env.ExpandPath(finalGCfg.Desktop.Wallpaper.Default)
	finalGCfg.Screenshot.Dir = env.ExpandPath(finalGCfg.Screenshot.Dir)
	finalGCfg.QuickSync.RepoDir = env.ExpandPath(finalGCfg.QuickSync.RepoDir)
	if err := driver.SetWeights(finalGCfg.Drivers); err != nil {
		return nil, err
	}
	return &Config{GlobalConfig: finalGCfg, raw: rawMerged}, nil
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

type ModuleMetadata struct {
	Requires   []string `json:"requires"`
	Recommends []string `json:"recommends"`
}

func validateDependencies(enabled map[string]bool, meta map[string]ModuleMetadata) error {
	for name := range enabled {
		m, ok := meta[name]
		if !ok {
			continue
		}
		for _, req := range m.Requires {
			if !enabled[req] {
				return fmt.Errorf("module %q requires %q, but it is not enabled", name, req)
			}
		}
		for _, rec := range m.Recommends {
			if !enabled[rec] {
				slog.Warn("module recommendation is not enabled", "module", name, "recommends", rec)
			}
		}
	}
	deps := make(map[string][]string)
	for name, m := range meta {
		deps[name] = m.Requires
	}
	return detectCycles(deps)
}

func detectCycles(deps map[string][]string) error {
	visited := make(map[string]bool)
	recStack := make(map[string]bool)
	var check func(node string) error
	check = func(node string) error {
		visited[node] = true
		recStack[node] = true
		for _, neighbor := range deps[node] {
			if !visited[neighbor] {
				if err := check(neighbor); err != nil {
					return err
				}
			} else if recStack[neighbor] {
				return fmt.Errorf("circular dependency detected involving module %q", neighbor)
			}
		}
		recStack[node] = false
		return nil
	}
	for node := range deps {
		if !visited[node] {
			if err := check(node); err != nil {
				return err
			}
		}
	}
	return nil
}

func MergeStrict(dst, src map[string]any, allowSubstitution bool) error {
	for k, v := range src {
		if v == nil {
			continue
		}
		if existing, ok := dst[k]; ok && existing != nil {
			if reflect.TypeOf(v).Kind() == reflect.Slice || reflect.TypeOf(v).Kind() == reflect.Array {
				return fmt.Errorf("lists are forbidden in strict config (key: %s)", k)
			}
			vVal := reflect.ValueOf(v)
			eVal := reflect.ValueOf(existing)
			if vVal.Kind() == reflect.Map && eVal.Kind() == reflect.Map {
				vMap := make(map[string]any)
				for _, key := range vVal.MapKeys() {
					vMap[key.String()] = vVal.MapIndex(key).Interface()
				}
				eMap := make(map[string]any)
				for _, key := range eVal.MapKeys() {
					eMap[key.String()] = eVal.MapIndex(key).Interface()
				}
				if err := MergeStrict(eMap, vMap, allowSubstitution); err != nil {
					return err
				}
				dst[k] = eMap
				continue
			}
			if !reflect.DeepEqual(existing, v) {
				if allowSubstitution {
					dst[k] = v
				} else {
					return fmt.Errorf("substitution forbidden: key %q already has value %v, cannot overwrite with %v", k, existing, v)
				}
			}
		} else {
			if reflect.TypeOf(v).Kind() == reflect.Slice || reflect.TypeOf(v).Kind() == reflect.Array {
				return fmt.Errorf("lists are forbidden in strict config (key: %s)", k)
			}
			dst[k] = v
		}
	}
	return nil
}
