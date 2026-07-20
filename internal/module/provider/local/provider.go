package local

import (
	"context"
	"errors"
	"fmt"
	"github.com/lucasew/workspaced/internal/module"
	"github.com/lucasew/workspaced/internal/modulecue"
	"os"
	"path/filepath"
	"strings"
)

var (
	ErrModuleNotFound           = errors.New("workspace module not found")
	ErrStrictStructureViolation = errors.New("strict structure violation: file found in module root")
	ErrUnknownPreset            = errors.New("unknown preset")
	ErrMissingModuleCue         = errors.New("module is missing module.cue")
)

func init() {
	module.RegisterProvider(&Provider{})
}

type Provider struct{}

func (p *Provider) ID() string   { return "self" }
func (p *Provider) Name() string { return "Workspace Module" }

var presetBases = map[string]string{
	"home": "~",
	"etc":  "/etc",
	"usr":  "/usr",
	"root": "/",
	"var":  "/var",
	"bin":  "/usr/local/bin",
}

func (p *Provider) Resolve(ctx context.Context, req module.ResolveRequest) ([]module.ResolvedFile, error) {
	workspaceRoot := filepath.Dir(req.ModulesBaseDir)
	modPath := req.Ref
	if !filepath.IsAbs(modPath) {
		modPath = filepath.Join(workspaceRoot, req.Ref)
	}
	if st, err := os.Stat(modPath); err != nil || !st.IsDir() {
		return nil, fmt.Errorf("%w: %q at %s", ErrModuleNotFound, req.Ref, modPath)
	}

	if err := validateConfig(req.Ref, modPath, req.ModuleConfig, req.Config.Raw()); err != nil {
		return nil, err
	}

	entries, err := os.ReadDir(modPath)
	if err != nil {
		return nil, err
	}

	var out []module.ResolvedFile
	for _, preset := range entries {
		if !preset.IsDir() {
			name := strings.TrimSpace(preset.Name())
			if name == "README.md" || strings.HasSuffix(name, ".cue") {
				continue
			}
			return nil, fmt.Errorf("%w: %q in module %q", ErrStrictStructureViolation, name, req.Ref)
		}
		presetName := preset.Name()
		targetBase, ok := presetBases[presetName]
		if !ok {
			return nil, fmt.Errorf("%w: %q in module %q", ErrUnknownPreset, presetName, req.Ref)
		}
		if targetBase == "~" {
			home, err := os.UserHomeDir()
			if err != nil {
				return nil, fmt.Errorf("get home directory: %w", err)
			}
			targetBase = home
		}

		presetPath := filepath.Join(modPath, presetName)
		err := filepath.Walk(presetPath, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}
			if info.IsDir() {
				return nil
			}
			rel, err := filepath.Rel(presetPath, path)
			if err != nil {
				return err
			}
			isSymlink := info.Mode()&os.ModeSymlink != 0
			out = append(out, module.ResolvedFile{
				RelPath:    rel,
				TargetBase: targetBase,
				Mode:       info.Mode(),
				Info:       fmt.Sprintf("module:%s (%s/%s)", req.Ref, presetName, rel),
				AbsPath:    path,
				Symlink:    isSymlink,
			})
			return nil
		})
		if err != nil {
			return nil, err
		}
	}

	return out, nil
}

func validateConfig(modName string, modPath string, modCfg map[string]any, root map[string]any) error {
	if !modulecue.Exists(modPath) {
		return fmt.Errorf("%w: %q", ErrMissingModuleCue, modName)
	}
	return modulecue.ValidateConfigWithRoot(modPath, modCfg, root)
}
