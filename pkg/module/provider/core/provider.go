package core

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"workspaced/pkg/env"
	"workspaced/pkg/icons"
	"workspaced/pkg/module"
)

func init() {
	module.RegisterProvider(&Provider{})
}

type Provider struct{}

func (p *Provider) ID() string   { return "core" }
func (p *Provider) Name() string { return "Core Module Provider" }

type base16IconsConfig struct {
	InputDir       string   `json:"input_dir"`
	OutputDir      string   `json:"output_dir"`
	ThemeName      string   `json:"theme_name"`
	Jobs           string   `json:"jobs"`
	Sizes          string   `json:"sizes"`
	Replace        []string `json:"replace"`
	MapScheme      bool     `json:"map_scheme"`
	NoRaster       bool     `json:"no_raster"`
	DefaultContext string   `json:"default_context"`
}

func (p *Provider) Resolve(ctx context.Context, req module.ResolveRequest) ([]module.ResolvedFile, error) {
	switch req.Ref {
	case "base16-icons-linux":
		return p.resolveBase16IconsLinux(ctx, req)
	default:
		return nil, fmt.Errorf("unknown core module %q", req.Ref)
	}
}

func (p *Provider) resolveBase16IconsLinux(ctx context.Context, req module.ResolveRequest) ([]module.ResolvedFile, error) {
	cfg := base16IconsConfig{
		ThemeName:      "workspaced-base16",
		Jobs:           "auto",
		Sizes:          "16,24,32,48,64,128,256",
		MapScheme:      true,
		DefaultContext: "apps",
	}

	b, err := json.Marshal(req.ModuleConfig)
	if err != nil {
		return nil, err
	}
	if err := json.Unmarshal(b, &cfg); err != nil {
		return nil, fmt.Errorf("invalid module config for %s: %w", req.ModuleName, err)
	}

	if strings.TrimSpace(cfg.InputDir) == "" {
		return nil, fmt.Errorf("input_dir is required for core:base16-icons-linux")
	}
	cfg.InputDir = env.ExpandPath(cfg.InputDir)
	if cfg.OutputDir == "" {
		cfg.OutputDir = filepath.Join("~/.local/share/icons", cfg.ThemeName)
	}
	cfg.OutputDir = env.ExpandPath(cfg.OutputDir)

	if _, err := os.Stat(cfg.InputDir); err != nil {
		return nil, fmt.Errorf("invalid input_dir %q: %w", cfg.InputDir, err)
	}
	if cfg.ThemeName == "" {
		return nil, fmt.Errorf("theme_name is required")
	}

	palette, err := extractBase16(req)
	if err != nil {
		return nil, err
	}
	fp, err := moduleFingerprint(cfg, palette)
	if err != nil {
		return nil, err
	}

	cacheRoot := env.ExpandPath("~/.cache/workspaced/modules/core-base16-icons-linux")
	cacheDir := filepath.Join(cacheRoot, fp)
	if _, err := os.Stat(filepath.Join(cacheDir, "index.theme")); os.IsNotExist(err) {
		if err := os.MkdirAll(cacheDir, 0755); err != nil {
			return nil, err
		}
		err := icons.RunThemeGenerate(ctx, icons.ThemeGenerateOptions{
			InputDir:       cfg.InputDir,
			OutputDir:      cacheDir,
			ThemeName:      cfg.ThemeName,
			Jobs:           cfg.Jobs,
			Sizes:          cfg.Sizes,
			Replace:        cfg.Replace,
			MapScheme:      cfg.MapScheme,
			HasMapScheme:   true,
			Clean:          true,
			NoRaster:       cfg.NoRaster,
			UpdateCache:    false,
			HasUpdateCache: true,
			DefaultContext: cfg.DefaultContext,
			UseCache:       false,
		})
		if err != nil {
			return nil, err
		}
	}

	var out []module.ResolvedFile
	err = filepath.WalkDir(cacheDir, func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if d.IsDir() {
			return nil
		}
		if strings.HasPrefix(filepath.Base(path), ".") {
			return nil
		}
		rel, err := filepath.Rel(cacheDir, path)
		if err != nil {
			return err
		}
		info, err := d.Info()
		if err != nil {
			return err
		}
		out = append(out, module.ResolvedFile{
			RelPath:    rel,
			TargetBase: cfg.OutputDir,
			Mode:       info.Mode(),
			Info:       fmt.Sprintf("module:%s (%s)", req.ModuleName, rel),
			AbsPath:    path,
			Symlink:    false,
		})
		return nil
	})
	if err != nil {
		return nil, err
	}
	sort.Slice(out, func(i, j int) bool { return out[i].RelPath < out[j].RelPath })
	return out, nil
}

func extractBase16(req module.ResolveRequest) (map[string]any, error) {
	raw, ok := req.Config.Modules["base16"]
	if !ok {
		return nil, fmt.Errorf("module %q from core:base16-icons-linux requires modules.base16", req.ModuleName)
	}
	m, ok := raw.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("invalid modules.base16 config")
	}
	return m, nil
}

func moduleFingerprint(cfg base16IconsConfig, palette map[string]any) (string, error) {
	stats, err := sourceStats(cfg.InputDir)
	if err != nil {
		return "", err
	}
	payload := map[string]any{
		"engine":          "core-base16-icons-linux-v1",
		"input_dir":       filepath.Clean(cfg.InputDir),
		"theme_name":      cfg.ThemeName,
		"jobs":            cfg.Jobs,
		"sizes":           cfg.Sizes,
		"replace":         cfg.Replace,
		"map_scheme":      cfg.MapScheme,
		"no_raster":       cfg.NoRaster,
		"default_context": cfg.DefaultContext,
		"stats":           stats,
		"palette":         palette,
	}
	b, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}
	h := sha256.Sum256(b)
	return hex.EncodeToString(h[:]), nil
}

func sourceStats(dir string) (map[string]any, error) {
	var count int64
	var size int64
	var maxMtime int64
	err := filepath.WalkDir(dir, func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if d.IsDir() {
			return nil
		}
		name := d.Name()
		if !strings.HasSuffix(name, ".svg") && !strings.HasSuffix(name, ".svg.tmpl") {
			return nil
		}
		info, err := d.Info()
		if err != nil {
			return err
		}
		count++
		size += info.Size()
		mt := info.ModTime().Unix()
		if mt > maxMtime {
			maxMtime = mt
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return map[string]any{"count": count, "size": size, "max_mtime": maxMtime}, nil
}
