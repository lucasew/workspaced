package local

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"workspaced/pkg/module"

	"github.com/xeipuuv/gojsonschema"
)

func init() {
	module.RegisterProvider(&Provider{})
}

type Provider struct{}

func (p *Provider) ID() string   { return "local" }
func (p *Provider) Name() string { return "Local Module" }

var presetBases = map[string]string{
	"home": "~",
	"etc":  "/etc",
	"usr":  "/usr",
	"root": "/",
	"var":  "/var",
	"bin":  "/usr/local/bin",
}

func (p *Provider) Resolve(ctx context.Context, req module.ResolveRequest) ([]module.ResolvedFile, error) {
	modPath := filepath.Join(req.ModulesBaseDir, req.Ref)
	if st, err := os.Stat(modPath); err != nil || !st.IsDir() {
		return nil, fmt.Errorf("local module %q not found at %s", req.Ref, modPath)
	}

	if err := validateConfig(req.Ref, modPath, req.ModuleConfig); err != nil {
		return nil, err
	}

	entries, err := os.ReadDir(modPath)
	if err != nil {
		return nil, err
	}

	var out []module.ResolvedFile
	for _, preset := range entries {
		if !preset.IsDir() {
			name := preset.Name()
			if name == "schema.json" || name == "module.toml" || name == "defaults.toml" || name == "README.md" {
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

func validateConfig(modName string, modPath string, modCfg map[string]any) error {
	schemaPath := filepath.Join(modPath, "schema.json")
	if _, err := os.Stat(schemaPath); os.IsNotExist(err) {
		return nil
	}
	absSchemaPath, err := filepath.Abs(schemaPath)
	if err != nil {
		return err
	}
	wrapperSchema := map[string]any{
		"type": "object",
		"properties": map[string]any{
			"enable": map[string]any{"type": "boolean"},
		},
		"required": []string{"enable"},
		"allOf":    []map[string]any{{"$ref": "file://" + absSchemaPath}},
	}
	schemaLoader := gojsonschema.NewGoLoader(wrapperSchema)
	documentLoader := gojsonschema.NewGoLoader(modCfg)
	result, err := gojsonschema.Validate(schemaLoader, documentLoader)
	if err != nil {
		return fmt.Errorf("failed to validate module %q: %w", modName, err)
	}
	if !result.Valid() {
		var errs strings.Builder
		for _, desc := range result.Errors() {
			fmt.Fprintf(&errs, "- %s\n", desc)
		}
		return fmt.Errorf("config validation failed for module %q:\n%s", modName, errs.String())
	}
	return nil
}
