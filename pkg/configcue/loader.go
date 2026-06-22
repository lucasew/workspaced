package configcue

import (
	"context"
	"crypto/sha256"
	"embed"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"

	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"

	"github.com/pbnjay/memory"

	"cuelang.org/go/cue/ast"
	"workspaced/pkg/driver"
	execdriver "workspaced/pkg/driver/exec"
	envdriver "workspaced/pkg/driver/env"
	"workspaced/pkg/logging"
	"workspaced/pkg/modulecue"

	"cuelang.org/go/cue"
	"cuelang.org/go/cue/cuecontext"
	cueerrors "cuelang.org/go/cue/errors"
	"cuelang.org/go/cue/format"
	"cuelang.org/go/cue/parser"
	"cuelang.org/go/cue/token"
)

var ErrInvalidInputSpec = errors.New("invalid input spec")

//go:embed schema.cue prelude_common.cue prelude_home.cue prelude_codebase.cue
var schemaFS embed.FS

type Layer struct {
	Name string `json:"name"`
	Path string `json:"path"`
}

type DiscoverOptions struct {
	Cwd      string
	HomeMode bool
}

type DiscoverResult struct {
	Layers []Layer `json:"layers"`
}

type EvaluationResult struct {
	Layers []Layer         `json:"layers"`
	JSON   json.RawMessage `json:"json"`
}

func DiscoverLayers(ctx context.Context, opts DiscoverOptions) (DiscoverResult, error) {
	layers := make([]Layer, 0)

	if !opts.HomeMode {
		repoPath, err := ResolveWorkspaceCuePath(ctx, opts.Cwd)
		if err != nil {
			return DiscoverResult{}, err
		}
		if repoPath != "" {
			layers = append(layers, Layer{Name: "repo", Path: repoPath})
		}
	}

	if opts.HomeMode {
		dotfilesRoot, err := envdriver.GetDotfilesRoot(ctx)
		if err == nil && dotfilesRoot != "" {
			p := filepath.Join(dotfilesRoot, "workspaced.cue")
			if fileExists(p) {
				layers = append(layers, Layer{Name: "dotfiles", Path: p})
			}
		}
	}

	if opts.HomeMode {
		homeDir, err := os.UserHomeDir()
		if err == nil && homeDir != "" {
			p := filepath.Join(homeDir, "workspaced.cue")
			if fileExists(p) {
				layers = append(layers, Layer{Name: "user", Path: p})
			}
		}
	}

	if opts.HomeMode {
		configDir, err := envdriver.GetConfigDir(ctx)
		if err == nil && configDir != "" {
			p := filepath.Join(configDir, "workspaced.cue")
			if fileExists(p) {
				layers = append(layers, Layer{Name: "home", Path: p})
			}
		}
	}

	return DiscoverResult{Layers: layers}, nil
}

func ExportJSON(ctx context.Context, opts DiscoverOptions) ([]byte, error) {
	result, err := Evaluate(ctx, opts)
	if err != nil {
		return nil, err
	}
	return result.JSON, nil
}

func ExportCUE(ctx context.Context, opts DiscoverOptions) ([]byte, error) {
	discovered, err := DiscoverLayers(ctx, opts)
	if err != nil {
		return nil, err
	}
	paths := make([]string, 0, len(discovered.Layers))
	for _, layer := range discovered.Layers {
		paths = append(paths, layer.Path)
	}

	configValue, err := buildWorkspacedValue(ctx, paths, discovered.Layers, opts.HomeMode)
	if err != nil {
		return nil, err
	}
	// Use a fresh root with logger for the (rare) diagnostic warnings in this
	// top-level export path. The real work ctx is not threaded into these
	// high-level CUE export helpers.
	return formatWorkspacedValue(ctx, configValue, paths, discovered.Layers)
}

func ExportDef(ctx context.Context, opts DiscoverOptions) ([]byte, error) {
	discovered, err := DiscoverLayers(ctx, opts)
	if err != nil {
		return nil, err
	}
	paths := make([]string, 0, len(discovered.Layers))
	for _, layer := range discovered.Layers {
		paths = append(paths, layer.Path)
	}

	configValue, err := buildWorkspacedValue(ctx, paths, discovered.Layers, opts.HomeMode)
	if err != nil {
		return nil, err
	}
	return formatWorkspacedDef(ctx, configValue, paths, discovered.Layers)
}

func Evaluate(ctx context.Context, opts DiscoverOptions) (EvaluationResult, error) {
	discovered, err := DiscoverLayers(ctx, opts)
	if err != nil {
		return EvaluationResult{}, err
	}
	paths := make([]string, 0, len(discovered.Layers))
	for _, layer := range discovered.Layers {
		paths = append(paths, layer.Path)
	}
	b, err := exportJSONFromPaths(ctx, paths, discovered.Layers, opts.HomeMode)
	if err != nil {
		return EvaluationResult{}, err
	}
	return EvaluationResult{
		Layers: discovered.Layers,
		JSON:   b,
	}, nil
}

func ExportJSONFromPaths(ctx context.Context, paths []string) ([]byte, error) {
	return exportJSONFromPaths(ctx, paths, nil, false)
}

func exportJSONFromPaths(ctx context.Context, paths []string, discovered []Layer, homeMode bool) ([]byte, error) {
	configValue, err := buildWorkspacedValue(ctx, paths, discovered, homeMode)
	if err != nil {
		return nil, err
	}
	return marshalWorkspacedValue(ctx, configValue, paths, discovered)
}

func buildWorkspacedValue(ctx context.Context, paths []string, discovered []Layer, homeMode bool) (cue.Value, error) {
	baseRuntimePrelude, err := buildRuntimePrelude(ctx, nil)
	if err != nil {
		return cue.Value{}, err
	}
	cueCtx := cuecontext.New()
	initialValue, err := compileWorkspacedValueWithContext(cueCtx, paths, baseRuntimePrelude, homeMode, nil, nil)
	if err != nil {
		return cue.Value{}, err
	}
	resolvedInputs, err := resolveRuntimeInputs(initialValue, paths, discovered)
	if err != nil {
		return cue.Value{}, err
	}
	runtimePrelude, err := buildRuntimePrelude(ctx, resolvedInputs)
	if err != nil {
		return cue.Value{}, err
	}
	baseConfigValue, err := compileWorkspacedValueWithContext(cueCtx, paths, runtimePrelude, homeMode, nil, nil)
	if err != nil {
		return cue.Value{}, err
	}
	preLayers, postLayers, err := buildResolvedModuleLayers(baseConfigValue, paths, discovered)
	if err != nil {
		return cue.Value{}, err
	}
	configValue, err := compileWorkspacedValueWithContext(cueCtx, paths, runtimePrelude, homeMode, preLayers, postLayers)
	if err != nil {
		return cue.Value{}, err
	}
	return configValue, nil
}

func compileWorkspacedValueWithContext(ctx *cue.Context, paths []string, runtimePrelude string, homeMode bool, preLayers []compiledLayer, postLayers []compiledLayer) (cue.Value, error) {
	schemaBytes, err := schemaFS.ReadFile("schema.cue")
	if err != nil {
		return cue.Value{}, fmt.Errorf("read embedded cue schema: %w", err)
	}
	preludeCommonBytes, err := schemaFS.ReadFile("prelude_common.cue")
	if err != nil {
		return cue.Value{}, fmt.Errorf("read embedded cue prelude_common: %w", err)
	}
	preludeVariantFile := "prelude_codebase.cue"
	if homeMode {
		preludeVariantFile = "prelude_home.cue"
	}
	preludeVariantBytes, err := schemaFS.ReadFile(preludeVariantFile)
	if err != nil {
		return cue.Value{}, fmt.Errorf("read embedded cue %s: %w", preludeVariantFile, err)
	}

	v := ctx.CompileString(string(schemaBytes), cue.Filename("schema.cue"))
	if err := v.Err(); err != nil {
		return cue.Value{}, fmt.Errorf("compile embedded cue schema:\n%s", cueerrors.Details(err, nil))
	}

	preludeCommonLayer := ctx.CompileString(string(preludeCommonBytes), cue.Filename("prelude_common.cue"))
	if err := preludeCommonLayer.Err(); err != nil {
		return cue.Value{}, fmt.Errorf("compile embedded cue prelude_common:\n%s", cueerrors.Details(err, nil))
	}
	v = v.Unify(preludeCommonLayer)
	if err := v.Err(); err != nil {
		return cue.Value{}, fmt.Errorf("unify embedded cue prelude_common:\n%s", cueerrors.Details(err, nil))
	}

	preludeVariantLayer := ctx.CompileString(string(preludeVariantBytes), cue.Filename(preludeVariantFile))
	if err := preludeVariantLayer.Err(); err != nil {
		return cue.Value{}, fmt.Errorf("compile embedded cue %s:\n%s", preludeVariantFile, cueerrors.Details(err, nil))
	}
	v = v.Unify(preludeVariantLayer)
	if err := v.Err(); err != nil {
		return cue.Value{}, fmt.Errorf("unify embedded cue %s:\n%s", preludeVariantFile, cueerrors.Details(err, nil))
	}

	runtimeLayer := ctx.CompileString(runtimePrelude, cue.Filename("runtime_prelude.cue"))
	if err := runtimeLayer.Err(); err != nil {
		return cue.Value{}, fmt.Errorf("compile runtime cue prelude:\n%s", cueerrors.Details(err, nil))
	}
	v = v.Unify(runtimeLayer)
	if err := v.Err(); err != nil {
		return cue.Value{}, fmt.Errorf("unify runtime cue prelude:\n%s", cueerrors.Details(err, nil))
	}

	driverLayer, err := buildDriverWeightLayer(v)
	if err != nil {
		return cue.Value{}, err
	}
	if driverLayer != "" {
		layerValue := ctx.CompileString(driverLayer, cue.Filename("driver_weights.cue"))
		if err := layerValue.Err(); err != nil {
			return cue.Value{}, fmt.Errorf("compile cue layer %s:\n%s", "driver_weights.cue", cueerrors.Details(err, nil))
		}
		v = v.Unify(layerValue)
		if err := v.Err(); err != nil {
			return cue.Value{}, fmt.Errorf("unify cue layer %s:\n%s", "driver_weights.cue", cueerrors.Details(err, nil))
		}
	}

	for _, layer := range preLayers {
		layerValue := ctx.CompileString(layer.Source, cue.Filename(layer.Name))
		if err := layerValue.Err(); err != nil {
			return cue.Value{}, fmt.Errorf("compile cue layer %s:\n%s", layer.Name, cueerrors.Details(err, nil))
		}
		v = v.Unify(layerValue)
		if err := v.Err(); err != nil {
			return cue.Value{}, fmt.Errorf("unify cue layer %s:\n%s", layer.Name, cueerrors.Details(err, nil))
		}
	}

	for _, path := range paths {
		src, err := os.ReadFile(path)
		if err != nil {
			return cue.Value{}, fmt.Errorf("read cue layer %s: %w", path, err)
		}
		layerValue := ctx.CompileString(string(src), cue.Filename(path))
		if err := layerValue.Err(); err != nil {
			return cue.Value{}, fmt.Errorf("compile cue layer %s: %w", path, err)
		}
		// Support both wrapped style (`workspaced: { modules: ... }`)
		// and bare style (top-level `modules: ...` etc. directly in the file).
		// This makes sure modules etc from the cue are always under the workspaced value.
		if ws := layerValue.LookupPath(cue.ParsePath("workspaced")); ws.Exists() {
			// wrapped style: the src itself starts with "workspaced: { ... }"
			// unify the full layerValue so the "workspaced" key merges properly
			v = v.Unify(layerValue)
		} else {
			// bare style (top-level modules, inputs etc. without the wrapper)
			// wrap by filling the bare value under workspaced
			template := ctx.CompileString(`workspaced: {}`)
			wrapped := template.Fill(layerValue, "workspaced")
			v = v.Unify(wrapped)
		}
		if err := v.Err(); err != nil {
			return cue.Value{}, fmt.Errorf("unify cue layer %s: %w", path, err)
		}
	}

	for _, layer := range postLayers {
		layerValue := ctx.CompileString(layer.Source, cue.Filename(layer.Name))
		if err := layerValue.Err(); err != nil {
			return cue.Value{}, fmt.Errorf("compile cue layer %s:\n%s", layer.Name, cueerrors.Details(err, nil))
		}
		v = v.Unify(layerValue)
		if err := v.Err(); err != nil {
			return cue.Value{}, fmt.Errorf("unify cue layer %s:\n%s", layer.Name, cueerrors.Details(err, nil))
		}
	}

	configValue := v.LookupPath(cue.ParsePath("workspaced"))
	if err := configValue.Err(); err != nil {
		return cue.Value{}, fmt.Errorf("lookup workspaced value:\n%s", cueerrors.Details(err, nil))
	}
	return configValue, nil
}

type compiledLayer struct {
	Name   string
	Source string
}

func buildResolvedModuleLayers(configValue cue.Value, paths []string, discovered []Layer) ([]compiledLayer, []compiledLayer, error) {
	configJSON, err := configValue.MarshalJSON()
	if err != nil {
		return nil, nil, fmt.Errorf("marshal cue config before module config resolution: %w", err)
	}
	var raw map[string]any
	if err := json.Unmarshal(configJSON, &raw); err != nil {
		return nil, nil, fmt.Errorf("decode cue config before module config resolution: %w", err)
	}
	cfg, err := decodeConfig(configJSON)
	if err != nil {
		return nil, nil, err
	}

	modules, err := cfg.Modules()
	if err != nil {
		return nil, nil, fmt.Errorf("decode modules from config: %w", err)
	}
	modulesBaseDir := resolveModulesBaseDir(paths, discovered)
	if modulesBaseDir == "" {
		return nil, nil, nil
	}

	schemaByModule := map[string]string{}
	for modName, modEntry := range modules {
		modulePath, ok, err := resolveLocalModulePath(cfg, modName, modEntry, modulesBaseDir)
		if err != nil {
			return nil, nil, fmt.Errorf("resolve module %q for config resolution: %w", modName, err)
		}
		if !ok {
			continue
		}
		if !modulecue.Exists(modulePath) {
			continue
		}
		schemaText, err := modulecue.ConfigSyntaxWithRoot(modulePath, raw)
		if err != nil {
			return nil, nil, err
		}
		schemaByModule[modName] = schemaText
	}

	preLayers := make([]compiledLayer, 0, 1)
	if len(schemaByModule) > 0 {
		schemaLayer, err := buildModuleSchemaLayer(schemaByModule)
		if err != nil {
			return nil, nil, err
		}
		preLayers = append(preLayers, compiledLayer{
			Name:   "module_schemas.cue",
			Source: schemaLayer,
		})
	}
	postLayers := make([]compiledLayer, 0, 1)
	if hasDerivedDesktopModules(raw) {
		postLayers = append(postLayers, compiledLayer{
			Name:   "derived_modules.cue",
			Source: buildDerivedModulePrelude(),
		})
	}
	return preLayers, postLayers, nil
}

func resolveModulesBaseDir(paths []string, discovered []Layer) string {
	for _, layer := range discovered {
		if layer.Name == "repo" || layer.Name == "dotfiles" {
			return filepath.Join(filepath.Dir(layer.Path), "modules")
		}
	}
	if len(paths) > 0 {
		return filepath.Join(filepath.Dir(paths[0]), "modules")
	}
	return ""
}

func hasDerivedDesktopModules(raw map[string]any) bool {
	modules, _ := raw["modules"].(map[string]any)
	if len(modules) == 0 {
		return false
	}
	_, hasBase16 := modules["base16"]
	_, hasGTK := modules["base16-gtk"]
	return hasBase16 || hasGTK
}

func buildDerivedModulePrelude() string {
	return `package workspaced

workspaced: {
	desktop: {
		dark_mode: *workspaced.modules.base16.config.dark_mode | bool
		raw: {
			dconf: *workspaced.modules["base16-gtk"].config.dconf | {
				[string]: [string]: _
			}
		}
	}
}
`
}

func buildModuleSchemaLayer(schemaByModule map[string]string) (string, error) {
	moduleNames := make([]string, 0, len(schemaByModule))
	for name := range schemaByModule {
		moduleNames = append(moduleNames, name)
	}
	sort.Strings(moduleNames)

	moduleFields := make([]ast.Decl, 0, len(moduleNames))
	for _, name := range moduleNames {
		expr, err := parser.ParseExpr(name+".module_config.cue", strings.TrimSpace(schemaByModule[name]))
		if err != nil {
			return "", fmt.Errorf("parse module config schema for %q: %w", name, err)
		}
		moduleFields = append(moduleFields, &ast.Field{
			Label: ast.NewString(name),
			Value: &ast.StructLit{
				Elts: []ast.Decl{
					&ast.Field{
						Label: ast.NewIdent("config"),
						Value: expr,
					},
				},
			},
		})
	}

	file := &ast.File{
		Decls: []ast.Decl{
			&ast.Package{Name: ast.NewIdent("workspaced")},
			&ast.Field{
				Label: ast.NewIdent("workspaced"),
				Value: &ast.StructLit{
					Elts: []ast.Decl{
						&ast.Field{
							Label: ast.NewIdent("modules"),
							Value: &ast.StructLit{
								Elts: moduleFields,
							},
						},
					},
				},
			},
		},
	}
	formatted, err := format.Node(file)
	if err != nil {
		return "", fmt.Errorf("format generated module schema layer: %w", err)
	}
	return string(formatted), nil
}

func buildDriverWeightLayer(current cue.Value) (string, error) {
	shape := driver.RegisteredWeightShape()
	if len(shape) == 0 {
		return "", nil
	}

	ifaceNames := make([]string, 0, len(shape))
	for name := range shape {
		ifaceNames = append(ifaceNames, name)
	}
	sort.Strings(ifaceNames)

	driverFields := make([]ast.Decl, 0, len(ifaceNames))
	for _, ifaceName := range ifaceNames {
		providerIDs := shape[ifaceName]
		providerFields := make([]ast.Decl, 0, len(providerIDs))
		for _, providerID := range providerIDs {
			if hasDriverWeight(current, ifaceName, providerID) {
				continue
			}
			providerFields = append(providerFields, &ast.Field{
				Label: ast.NewString(providerID),
				Value: ast.NewBinExpr(
					token.OR,
					&ast.UnaryExpr{
						Op: token.MUL,
						X:  ast.NewLit(token.INT, strconv.Itoa(50)),
					},
					ast.NewIdent("int"),
				),
			})
		}
		if len(providerFields) == 0 {
			continue
		}
		driverFields = append(driverFields, &ast.Field{
			Label: ast.NewString(ifaceName),
			Value: &ast.StructLit{Elts: providerFields},
		})
	}
	if len(driverFields) == 0 {
		return "", nil
	}

	file := &ast.File{
		Decls: []ast.Decl{
			&ast.Package{Name: ast.NewIdent("workspaced")},
			&ast.Field{
				Label: ast.NewIdent("workspaced"),
				Value: ast.NewStruct(
					ast.NewIdent("drivers"), &ast.StructLit{Elts: driverFields},
				),
			},
		},
	}
	formatted, err := format.Node(file)
	if err != nil {
		return "", fmt.Errorf("format generated driver weight layer: %w", err)
	}
	return string(formatted), nil
}

func hasDriverWeight(v cue.Value, ifaceName string, providerID string) bool {
	path := cue.MakePath(
		cue.Str("workspaced"),
		cue.Str("drivers"),
		cue.Str(ifaceName),
		cue.Str(providerID),
	)
	current := v.LookupPath(path)
	return current.Exists() && current.Err() == nil
}

func resolveLocalModulePath(cfg *Config, moduleName string, modEntry ModuleEntry, modulesBaseDir string) (string, bool, error) {
	workspaceRoot := filepath.Dir(modulesBaseDir)
	modulePath := strings.Trim(strings.TrimSpace(modEntry.Path), "/")
	if modulePath == "" {
		modulePath = filepath.ToSlash(filepath.Join("modules", moduleName))
	}

	if from := strings.TrimSpace(modEntry.From); from != "" {
		return resolveLocalSourceSpec(workspaceRoot, modulePath, from)
	}

	inputName := strings.TrimSpace(modEntry.Input)
	if inputName == "" {
		inputName = "self"
	}
	if strings.Contains(inputName, ":") {
		return resolveLocalSourceSpec(workspaceRoot, modulePath, inputName)
	}
	if inputName == "self" {
		return filepath.Join(workspaceRoot, filepath.FromSlash(modulePath)), true, nil
	}

	inputs, err := cfg.Inputs()
	if err != nil {
		return "", false, err
	}
	input, ok := inputs[inputName]
	if !ok {
		return "", false, nil
	}
	return resolveLocalSourceSpec(workspaceRoot, modulePath, input.From)
}

func resolveLocalSourceSpec(workspaceRoot, modulePath, spec string) (string, bool, error) {
	spec = strings.TrimSpace(spec)
	if spec == "self" {
		return filepath.Join(workspaceRoot, filepath.FromSlash(modulePath)), true, nil
	}
	if !strings.HasPrefix(spec, "self:") {
		return "", false, nil
	}
	ref := strings.TrimSpace(strings.TrimPrefix(spec, "self:"))
	if ref == "" {
		return filepath.Join(workspaceRoot, filepath.FromSlash(modulePath)), true, nil
	}
	if modulePath != "" && modulePath != filepath.ToSlash(filepath.Join("modules", filepath.Base(modulePath))) {
		ref = strings.TrimRight(ref, "/") + "/" + strings.TrimLeft(modulePath, "/")
	}
	return filepath.Join(workspaceRoot, filepath.FromSlash(ref)), true, nil
}

func marshalWorkspacedValue(ctx context.Context, configValue cue.Value, paths []string, discovered []Layer) ([]byte, error) {
	if !configValue.Exists() {
		if len(discovered) > 0 {
			logger := logging.GetLogger(ctx)
			logger.Warn("experimental cue export produced empty result", "reason", "missing workspaced field", "layers", discovered)
		} else if len(paths) > 0 {
			logger := logging.GetLogger(ctx)
			logger.Warn("experimental cue export produced empty result", "reason", "missing workspaced field", "paths", paths)
		}
		return json.Marshal(map[string]any{})
	}
	b, err := configValue.MarshalJSON()
	if err != nil {
		return nil, fmt.Errorf("marshal cue config to json: %w", err)
	}
	if string(b) == "{}" && len(discovered) > 0 {
		logger := logging.GetLogger(ctx)
		logger.Warn("experimental cue export produced empty result", "reason", "workspaced resolved to empty object", "layers", discovered)
	} else if string(b) == "{}" && len(paths) > 0 {
		logger := logging.GetLogger(ctx)
		logger.Warn("experimental cue export produced empty result", "reason", "workspaced resolved to empty object", "paths", paths)
	}
	return b, nil
}

func formatWorkspacedValue(ctx context.Context, configValue cue.Value, paths []string, discovered []Layer) ([]byte, error) {
	if !configValue.Exists() {
		if len(discovered) > 0 {
			logger := logging.GetLogger(ctx)
			logger.Warn("experimental cue export produced empty result", "reason", "missing workspaced field", "layers", discovered)
		} else if len(paths) > 0 {
			logger := logging.GetLogger(ctx)
			logger.Warn("experimental cue export produced empty result", "reason", "missing workspaced field", "paths", paths)
		}
		return []byte("{}\n"), nil
	}

	n := configValue.Syntax(
		cue.Concrete(false),
		cue.Final(),
		cue.Definitions(false),
		cue.Hidden(false),
		cue.Optional(false),
		cue.Attributes(false),
		cue.Docs(false),
	)
	out, err := format.Node(n, format.Simplify())
	if err != nil {
		return nil, fmt.Errorf("format cue config: %w", err)
	}
	return append(out, '\n'), nil
}

func formatWorkspacedDef(ctx context.Context, configValue cue.Value, paths []string, discovered []Layer) ([]byte, error) {
	if !configValue.Exists() {
		if len(discovered) > 0 {
			logger := logging.GetLogger(ctx)
			logger.Warn("experimental cue def produced empty result", "reason", "missing workspaced field", "layers", discovered)
		} else if len(paths) > 0 {
			logger := logging.GetLogger(ctx)
			logger.Warn("experimental cue def produced empty result", "reason", "missing workspaced field", "paths", paths)
		}
		return []byte("{}\n"), nil
	}

	n := configValue.Syntax(
		cue.Concrete(false),
		cue.Definitions(true),
		cue.Hidden(false),
		cue.Optional(true),
		cue.Attributes(true),
		cue.Docs(true),
	)
	out, err := format.Node(n, format.Simplify())
	if err != nil {
		return nil, fmt.Errorf("format cue def: %w", err)
	}
	return append(out, '\n'), nil
}

func findUp(ctx context.Context, start string, name string) (string, error) {
	if start == "" {
		var err error
		start, err = os.Getwd()
		if err != nil {
			return "", err
		}
	}

	dir, err := filepath.Abs(start)
	if err != nil {
		return "", err
	}

	// Determine the git root of the starting point (if any). We will not
	// walk above it when looking for workspaced.cue. This ensures nested
	// git repos don't see outer workspaced.cue files.
	gitRoot, gitErr := getGitRoot(ctx, dir)
	hasGitBoundary := gitErr == nil && gitRoot != ""
	var absGit string
	if hasGitBoundary {
		absGit, _ = filepath.Abs(gitRoot)
		absGit = filepath.Clean(absGit)
	}

	for {
		candidate := filepath.Join(dir, name)
		if fileExists(candidate) {
			return candidate, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", nil
		}
		if hasGitBoundary {
			absParent, _ := filepath.Abs(parent)
			absParent = filepath.Clean(absParent)
			if absParent != absGit && !strings.HasPrefix(absParent, absGit+string(filepath.Separator)) {
				// would leave the git repo root; stop without considering parent
				return "", nil
			}
		}
		dir = parent
	}
}

func ResolveWorkspaceCuePath(ctx context.Context, start string) (string, error) {
	if start == "" {
		var err error
		start, err = os.Getwd()
		if err != nil {
			return "", err
		}
	}

	// Walk up from the starting directory to find the *closest* workspaced.cue,
	// but stop at the git root of the starting dir. This supports sub-workspaces
	// (cues deeper in the tree) inside a git repo, while ensuring that a git repo
	// nested inside another git repo does not inherit the parent's workspaced.cue.
	return findUp(ctx, start, "workspaced.cue")
}

func getGitRoot(ctx context.Context, path string) (string, error) {
	if _, err := os.Stat(path); err != nil {
		return "", err
	}
	cmd := execdriver.MustRun(ctx, "git", "-C", path, "rev-parse", "--show-toplevel")
	out, err := cmd.Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}

func fileExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}

func buildRuntimePrelude(ctx context.Context, resolvedInputs map[string]map[string]any) (string, error) {
	home, _ := os.UserHomeDir()
	dotfilesRoot, _ := envdriver.GetDotfilesRoot(ctx)
	configDir, _ := envdriver.GetConfigDir(ctx)
	userDataDir, _ := envdriver.GetUserDataDir(ctx)
	hostname, _ := envdriver.GetHostname(ctx)

	runtimeMap := map[string]any{
		"is_phone":      envdriver.IsPhone(ctx),
		"hostname":      hostname,
		"home":          home,
		"dotfiles_root": dotfilesRoot,
		"config_dir":    configDir,
		"user_data_dir": userDataDir,
		"cpus":          runtime.NumCPU(),
		"goos":          runtime.GOOS,
		"goarch":        runtime.GOARCH,
		"memory":        memory.TotalMemory(),
	}
	if len(resolvedInputs) > 0 {
		runtimeMap["inputs"] = resolvedInputs
	}

	payload := map[string]any{
		"workspaced": map[string]any{
			"runtime": runtimeMap,
		},
	}
	b, err := json.Marshal(payload)
	if err != nil {
		return "", fmt.Errorf("marshal runtime cue prelude: %w", err)
	}
	return string(b), nil
}

func resolveRuntimeInputs(configValue cue.Value, paths []string, discovered []Layer) (map[string]map[string]any, error) {
	b, err := configValue.MarshalJSON()
	if err != nil {
		return nil, fmt.Errorf("marshal cue config for input resolution: %w", err)
	}
	var raw map[string]any
	if err := json.Unmarshal(b, &raw); err != nil {
		return nil, fmt.Errorf("decode cue config for input resolution: %w", err)
	}
	inputsRaw, _ := raw["inputs"].(map[string]any)
	if len(inputsRaw) == 0 {
		return nil, nil
	}

	type inputCfg struct {
		From    string `json:"from"`
		Version string `json:"version"`
	}
	cfgInputs := map[string]inputCfg{}
	tmp, _ := json.Marshal(inputsRaw)
	_ = json.Unmarshal(tmp, &cfgInputs)

	modulesBaseDir := resolvedSelfModulesBase(paths, discovered)
	workspaceRoot := filepath.Dir(modulesBaseDir)
	home, _ := os.UserHomeDir()
	out := map[string]map[string]any{}
	for name, input := range cfgInputs {
		spec := strings.TrimSpace(input.From)
		if spec == "" {
			continue
		}
		if spec == "self" || name == "self" {
			out[name] = map[string]any{"path": workspaceRoot}
			continue
		}

		provider, target, ok := parseInputSpec(spec)
		if !ok {
			return nil, fmt.Errorf("resolve runtime input %q: %w: %q", name, ErrInvalidInputSpec, spec)
		}
		switch provider {
		case "github":
			cacheKey := githubCacheKey(target, input.Version)
			out[name] = map[string]any{
				"path": filepath.Join(home, ".cache", "workspaced", "sources", "github", hashPath(cacheKey)),
			}
		case "local":
			base := target
			if !filepath.IsAbs(base) {
				base = filepath.Join(workspaceRoot, base)
			}
			out[name] = map[string]any{"path": filepath.Clean(base)}
		default:
			out[name] = map[string]any{
				"provider": provider,
				"target":   target,
			}
		}
	}
	return out, nil
}

func resolvedSelfModulesBase(paths []string, discovered []Layer) string {
	for _, layer := range discovered {
		if layer.Name == "repo" || layer.Name == "dotfiles" {
			return filepath.Join(filepath.Dir(layer.Path), "modules")
		}
	}
	for _, path := range paths {
		return filepath.Join(filepath.Dir(path), "modules")
	}
	return filepath.Join(".", "modules")
}

func parseInputSpec(spec string) (string, string, bool) {
	parts := strings.SplitN(strings.TrimSpace(spec), ":", 2)
	if len(parts) != 2 {
		return "", "", false
	}
	provider := strings.TrimSpace(parts[0])
	target := strings.TrimSpace(parts[1])
	if provider == "" || target == "" {
		return "", "", false
	}
	return provider, target, true
}

func githubCacheKey(repo, version string) string {
	ref := strings.TrimSpace(version)
	if ref == "" {
		ref = "HEAD"
	}
	return "v4:repo:" + strings.Trim(strings.TrimSpace(repo), "/") + "@" + ref
}

func hashPath(key string) string {
	sum := sha256.Sum256([]byte(key))
	return hex.EncodeToString(sum[:])
}
