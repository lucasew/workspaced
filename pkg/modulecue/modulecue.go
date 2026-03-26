package modulecue

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"cuelang.org/go/cue"
	"cuelang.org/go/cue/cuecontext"
	"cuelang.org/go/cue/format"
)

type Metadata struct {
	Requires   []string `json:"requires"`
	Recommends []string `json:"recommends"`
}

type Definition struct {
	Meta    Metadata                  `json:"meta"`
	Config  map[string]any            `json:"config"`
	Drivers map[string]map[string]int `json:"drivers"`
}

func FilePath(modPath string) string {
	return filepath.Join(modPath, "module.cue")
}

func Exists(modPath string) bool {
	info, err := os.Stat(FilePath(modPath))
	return err == nil && !info.IsDir()
}

func Load(modPath string) (*Definition, error) {
	v, err := compileModule(modPath)
	if err != nil {
		return nil, err
	}

	moduleValue := v.LookupPath(cue.ParsePath("module"))
	if err := moduleValue.Err(); err != nil {
		return nil, fmt.Errorf("lookup module in %s: %w", FilePath(modPath), err)
	}

	data, err := moduleValue.MarshalJSON()
	if err != nil {
		return nil, fmt.Errorf("marshal module %s: %w", FilePath(modPath), err)
	}

	var def Definition
	if err := json.Unmarshal(data, &def); err != nil {
		return nil, fmt.Errorf("decode module %s: %w", FilePath(modPath), err)
	}
	if def.Config == nil {
		def.Config = make(map[string]any)
	}
	if def.Drivers == nil {
		def.Drivers = make(map[string]map[string]int)
	}
	return &def, nil
}

func ValidateConfig(modPath string, cfg map[string]any) error {
	_, err := ResolveConfig(modPath, cfg)
	return err
}

func ResolveConfig(modPath string, cfg map[string]any) (map[string]any, error) {
	ctx := cuecontext.New()
	v, err := compileModuleWithContext(ctx, modPath)
	if err != nil {
		return nil, err
	}

	configSchema := v.LookupPath(cue.ParsePath("module.config"))
	if err := configSchema.Err(); err != nil {
		return nil, fmt.Errorf("lookup module.config in %s: %w", FilePath(modPath), err)
	}

	if cfg == nil {
		cfg = make(map[string]any)
	}
	data, err := json.Marshal(cfg)
	if err != nil {
		return nil, fmt.Errorf("marshal module config for %s: %w", modPath, err)
	}

	cfgValue := ctx.CompileBytes(data, cue.Filename("module-config.json"))
	if err := cfgValue.Err(); err != nil {
		return nil, fmt.Errorf("compile module config for %s: %w", modPath, err)
	}

	unified := configSchema.Unify(cfgValue)
	if err := unified.Validate(cue.Concrete(true)); err != nil {
		return nil, fmt.Errorf("config validation failed for module %q: %w", filepath.Base(modPath), err)
	}

	resolvedJSON, err := unified.MarshalJSON()
	if err != nil {
		return nil, fmt.Errorf("marshal resolved module config for %s: %w", modPath, err)
	}

	resolved := map[string]any{}
	if err := json.Unmarshal(resolvedJSON, &resolved); err != nil {
		return nil, fmt.Errorf("decode resolved module config for %s: %w", modPath, err)
	}
	return resolved, nil
}

func ConfigSyntax(modPath string) (string, error) {
	v, err := compileModule(modPath)
	if err != nil {
		return "", err
	}

	configSchema := v.LookupPath(cue.ParsePath("module.config"))
	if err := configSchema.Err(); err != nil {
		return "", fmt.Errorf("lookup module.config in %s: %w", FilePath(modPath), err)
	}

	node := configSchema.Syntax(
		cue.Concrete(false),
		cue.Definitions(true),
		cue.Optional(true),
		cue.Attributes(true),
		cue.Docs(true),
	)
	formatted, err := format.Node(node)
	if err != nil {
		return "", fmt.Errorf("format module.config syntax for %s: %w", modPath, err)
	}
	return string(formatted), nil
}

func compileModule(modPath string) (cue.Value, error) {
	return compileModuleWithContext(cuecontext.New(), modPath)
}

func compileModuleWithContext(ctx *cue.Context, modPath string) (cue.Value, error) {
	path := FilePath(modPath)
	src, err := os.ReadFile(path)
	if err != nil {
		return cue.Value{}, fmt.Errorf("read %s: %w", path, err)
	}

	v := ctx.CompileBytes(src, cue.Filename(path))
	if err := v.Err(); err != nil {
		return cue.Value{}, fmt.Errorf("compile %s: %w", path, err)
	}
	return v, nil
}
