package modulecue

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"cuelang.org/go/cue"
	"cuelang.org/go/cue/ast"
	"cuelang.org/go/cue/build"
	"cuelang.org/go/cue/cuecontext"
	"cuelang.org/go/cue/format"
	"cuelang.org/go/cue/parser"
	"cuelang.org/go/cue/token"
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
	return ResolveConfigWithRoot(modPath, cfg, nil)
}

func ValidateConfigWithRoot(modPath string, cfg map[string]any, root map[string]any) error {
	_, err := ResolveConfigWithRoot(modPath, cfg, root)
	return err
}

func ResolveConfigWithRoot(modPath string, cfg map[string]any, root map[string]any) (map[string]any, error) {
	ctx := cuecontext.New()
	v, err := compileModuleWithContext(ctx, modPath, root)
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
	return ConfigSyntaxWithRoot(modPath, nil)
}

func ConfigSyntaxWithRoot(modPath string, root map[string]any) (string, error) {
	path := FilePath(modPath)
	src, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("read %s: %w", path, err)
	}
	file, err := parser.ParseFile(path, src)
	if err != nil {
		return "", fmt.Errorf("parse %s: %w", path, err)
	}
	configExpr, err := findModuleConfigExpr(file)
	if err != nil {
		return "", fmt.Errorf("lookup module.config in %s: %w", path, err)
	}
	formatted, err := format.Node(configExpr)
	if err != nil {
		return "", fmt.Errorf("format module.config syntax for %s: %w", modPath, err)
	}
	return string(formatted), nil
}

func compileModule(modPath string) (cue.Value, error) {
	return compileModuleWithContext(cuecontext.New(), modPath, nil)
}

func compileModuleWithContext(ctx *cue.Context, modPath string, root map[string]any) (cue.Value, error) {
	path := FilePath(modPath)
	moduleFile, err := parser.ParseFile(path, nil)
	if err != nil {
		return cue.Value{}, fmt.Errorf("parse %s: %w", path, err)
	}
	inst := &build.Instance{
		PkgName: "module",
		User:    true,
	}
	if err := inst.AddSyntax(moduleFile); err != nil {
		return cue.Value{}, fmt.Errorf("add %s to build instance: %w", path, err)
	}
	contextFile, err := buildContextFile(root)
	if err != nil {
		return cue.Value{}, fmt.Errorf("build module context for %s: %w", path, err)
	}
	if err := inst.AddSyntax(contextFile); err != nil {
		return cue.Value{}, fmt.Errorf("add module context for %s: %w", path, err)
	}
	v := ctx.BuildInstance(inst)
	if err := v.Err(); err != nil {
		return cue.Value{}, fmt.Errorf("compile %s: %w", path, err)
	}
	return v, nil
}

func buildContextFile(root map[string]any) (*ast.File, error) {
	rootExpr, err := cueExprFromAny(root)
	if err != nil {
		return nil, err
	}
	return &ast.File{
		Filename: "module_context.cue",
		Decls: []ast.Decl{
			&ast.Package{Name: ast.NewIdent("module")},
			&ast.Field{
				Label: ast.NewIdent("workspaced"),
				Value: rootExpr,
			},
		},
	}, nil
}

func cueExprFromAny(v any) (ast.Expr, error) {
	switch x := v.(type) {
	case nil:
		return ast.NewNull(), nil
	case bool:
		return ast.NewBool(x), nil
	case string:
		return ast.NewString(x), nil
	case json.Number:
		return numberExpr(string(x))
	case float64:
		return numberExpr(fmt.Sprintf("%v", x))
	case float32:
		return numberExpr(fmt.Sprintf("%v", x))
	case int:
		return ast.NewLit(token.INT, fmt.Sprintf("%d", x)), nil
	case int8:
		return ast.NewLit(token.INT, fmt.Sprintf("%d", x)), nil
	case int16:
		return ast.NewLit(token.INT, fmt.Sprintf("%d", x)), nil
	case int32:
		return ast.NewLit(token.INT, fmt.Sprintf("%d", x)), nil
	case int64:
		return ast.NewLit(token.INT, fmt.Sprintf("%d", x)), nil
	case uint:
		return ast.NewLit(token.INT, fmt.Sprintf("%d", x)), nil
	case uint8:
		return ast.NewLit(token.INT, fmt.Sprintf("%d", x)), nil
	case uint16:
		return ast.NewLit(token.INT, fmt.Sprintf("%d", x)), nil
	case uint32:
		return ast.NewLit(token.INT, fmt.Sprintf("%d", x)), nil
	case uint64:
		return ast.NewLit(token.INT, fmt.Sprintf("%d", x)), nil
	case map[string]any:
		keys := make([]string, 0, len(x))
		for k := range x {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		decls := make([]ast.Decl, 0, len(keys))
		for _, k := range keys {
			valueExpr, err := cueExprFromAny(x[k])
			if err != nil {
				return nil, err
			}
			decls = append(decls, &ast.Field{
				Label: ast.NewString(k),
				Value: valueExpr,
			})
		}
		return &ast.StructLit{Elts: decls}, nil
	case []any:
		exprs := make([]ast.Expr, 0, len(x))
		for _, item := range x {
			itemExpr, err := cueExprFromAny(item)
			if err != nil {
				return nil, err
			}
			exprs = append(exprs, itemExpr)
		}
		return ast.NewList(exprs...), nil
	default:
		return nil, fmt.Errorf("unsupported cue context value type %T", v)
	}
}

func numberExpr(s string) (ast.Expr, error) {
	if strings.ContainsAny(s, ".eE") {
		return ast.NewLit(token.FLOAT, s), nil
	}
	return ast.NewLit(token.INT, s), nil
}

func findModuleConfigExpr(file *ast.File) (ast.Expr, error) {
	moduleField := findField(file.Decls, "module")
	if moduleField == nil {
		return nil, fmt.Errorf("module field not found")
	}
	moduleStruct, ok := moduleField.Value.(*ast.StructLit)
	if !ok {
		return nil, fmt.Errorf("module field is not a struct")
	}
	configField := findField(moduleStruct.Elts, "config")
	if configField == nil {
		return nil, fmt.Errorf("module.config field not found")
	}
	return configField.Value, nil
}

func findField(decls []ast.Decl, name string) *ast.Field {
	for _, decl := range decls {
		field, ok := decl.(*ast.Field)
		if !ok {
			continue
		}
		label, isIdent, err := ast.LabelName(field.Label)
		if err == nil && isIdent && label == name {
			return field
		}
	}
	return nil
}
