package local

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"workspaced/pkg/module"
	"workspaced/pkg/modulecue"
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
		return nil, fmt.Errorf("workspace module %q not found at %s", req.Ref, modPath)
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
			return nil, fmt.Errorf("strict structure violation: file %q found in module %q root", name, req.Ref)
		}
		presetName := preset.Name()
		targetBase, ok := presetBases[presetName]
		if !ok {
			return nil, fmt.Errorf("unknown preset %q in module %q", presetName, req.Ref)
		}
		if targetBase == "~" {
			home, _ := os.UserHomeDir()
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
			rel, _ := filepath.Rel(presetPath, path)
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
		return fmt.Errorf("module %q is missing module.cue", modName)
	}
	return modulecue.ValidateConfigWithRoot(modPath, modCfg, root)
}
