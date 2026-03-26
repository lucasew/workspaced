package configcue

import (
	"context"
	"crypto/sha256"
	"embed"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	execdriver "workspaced/pkg/driver/exec"
	"workspaced/pkg/env"

	"cuelang.org/go/cue"
	"cuelang.org/go/cue/cuecontext"
)

//go:embed schema.cue prelude.cue
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

func DiscoverLayers(opts DiscoverOptions) (DiscoverResult, error) {
	layers := make([]Layer, 0)

	if !opts.HomeMode {
		repoPath, err := resolveWorkspaceCuePath(opts.Cwd)
		if err != nil {
			return DiscoverResult{}, err
		}
		if repoPath != "" {
			layers = append(layers, Layer{Name: "repo", Path: repoPath})
		}
	}

	if opts.HomeMode {
		dotfilesRoot, err := env.GetDotfilesRoot()
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
		configDir, err := env.GetConfigDir()
		if err == nil && configDir != "" {
			p := filepath.Join(configDir, "workspaced.cue")
			if fileExists(p) {
				layers = append(layers, Layer{Name: "home", Path: p})
			}
		}
	}

	return DiscoverResult{Layers: layers}, nil
}

func ExportJSON(opts DiscoverOptions) ([]byte, error) {
	result, err := Evaluate(opts)
	if err != nil {
		return nil, err
	}
	return result.JSON, nil
}

func Evaluate(opts DiscoverOptions) (EvaluationResult, error) {
	discovered, err := DiscoverLayers(opts)
	if err != nil {
		return EvaluationResult{}, err
	}
	paths := make([]string, 0, len(discovered.Layers))
	for _, layer := range discovered.Layers {
		paths = append(paths, layer.Path)
	}
	b, err := exportJSONFromPaths(paths, discovered.Layers)
	if err != nil {
		return EvaluationResult{}, err
	}
	return EvaluationResult{
		Layers: discovered.Layers,
		JSON:   b,
	}, nil
}

func ExportJSONFromPaths(paths []string) ([]byte, error) {
	return exportJSONFromPaths(paths, nil)
}

func exportJSONFromPaths(paths []string, discovered []Layer) ([]byte, error) {
	baseRuntimePrelude, err := buildRuntimePrelude(nil)
	if err != nil {
		return nil, err
	}
	initialValue, err := compileWorkspacedValue(paths, baseRuntimePrelude)
	if err != nil {
		return nil, err
	}
	resolvedInputs, err := resolveRuntimeInputs(initialValue, paths, discovered)
	if err != nil {
		return nil, err
	}
	runtimePrelude, err := buildRuntimePrelude(resolvedInputs)
	if err != nil {
		return nil, err
	}
	configValue, err := compileWorkspacedValue(paths, runtimePrelude)
	if err != nil {
		return nil, err
	}
	return marshalWorkspacedValue(configValue, paths, discovered)
}

func compileWorkspacedValue(paths []string, runtimePrelude string) (cue.Value, error) {
	ctx := cuecontext.New()
	schemaBytes, err := schemaFS.ReadFile("schema.cue")
	if err != nil {
		return cue.Value{}, fmt.Errorf("read embedded cue schema: %w", err)
	}
	preludeBytes, err := schemaFS.ReadFile("prelude.cue")
	if err != nil {
		return cue.Value{}, fmt.Errorf("read embedded cue prelude: %w", err)
	}

	v := ctx.CompileString(string(schemaBytes), cue.Filename("schema.cue"))
	if err := v.Err(); err != nil {
		return cue.Value{}, fmt.Errorf("compile embedded cue schema: %w", err)
	}

	preludeLayer := ctx.CompileString(string(preludeBytes), cue.Filename("prelude.cue"))
	if err := preludeLayer.Err(); err != nil {
		return cue.Value{}, fmt.Errorf("compile embedded cue prelude: %w", err)
	}
	v = v.Unify(preludeLayer)
	if err := v.Err(); err != nil {
		return cue.Value{}, fmt.Errorf("unify embedded cue prelude: %w", err)
	}

	runtimeLayer := ctx.CompileString(runtimePrelude, cue.Filename("runtime_prelude.cue"))
	if err := runtimeLayer.Err(); err != nil {
		return cue.Value{}, fmt.Errorf("compile runtime cue prelude: %w", err)
	}
	v = v.Unify(runtimeLayer)
	if err := v.Err(); err != nil {
		return cue.Value{}, fmt.Errorf("unify runtime cue prelude: %w", err)
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
		v = v.Unify(layerValue)
		if err := v.Err(); err != nil {
			return cue.Value{}, fmt.Errorf("unify cue layer %s: %w", path, err)
		}
	}

	configValue := v.LookupPath(cue.ParsePath("workspaced"))
	if err := configValue.Err(); err != nil {
		return cue.Value{}, fmt.Errorf("lookup workspaced value: %w", err)
	}
	return configValue, nil
}

func marshalWorkspacedValue(configValue cue.Value, paths []string, discovered []Layer) ([]byte, error) {
	if !configValue.Exists() {
		if len(discovered) > 0 {
			slog.Warn("experimental cue export produced empty result", "reason", "missing workspaced field", "layers", discovered)
		} else if len(paths) > 0 {
			slog.Warn("experimental cue export produced empty result", "reason", "missing workspaced field", "paths", paths)
		}
		return json.Marshal(map[string]any{})
	}
	b, err := configValue.MarshalJSON()
	if err != nil {
		return nil, fmt.Errorf("marshal cue config to json: %w", err)
	}
	if string(b) == "{}" && len(discovered) > 0 {
		slog.Warn("experimental cue export produced empty result", "reason", "workspaced resolved to empty object", "layers", discovered)
	} else if string(b) == "{}" && len(paths) > 0 {
		slog.Warn("experimental cue export produced empty result", "reason", "workspaced resolved to empty object", "paths", paths)
	}
	return b, nil
}

func findUp(start string, name string) (string, error) {
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

	for {
		candidate := filepath.Join(dir, name)
		if fileExists(candidate) {
			return candidate, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", nil
		}
		dir = parent
	}
}

func resolveWorkspaceCuePath(start string) (string, error) {
	if start == "" {
		var err error
		start, err = os.Getwd()
		if err != nil {
			return "", err
		}
	}

	if root, err := getGitRoot(start); err == nil && root != "" {
		candidate := filepath.Join(root, "workspaced.cue")
		if fileExists(candidate) {
			return candidate, nil
		}
		return "", nil
	}

	return findUp(start, "workspaced.cue")
}

func getGitRoot(path string) (string, error) {
	if _, err := os.Stat(path); err != nil {
		return "", err
	}
	cmd := execdriver.MustRun(context.Background(), "git", "-C", path, "rev-parse", "--show-toplevel")
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

func buildRuntimePrelude(resolvedInputs map[string]map[string]any) (string, error) {
	home, _ := os.UserHomeDir()
	dotfilesRoot, _ := env.GetDotfilesRoot()
	configDir, _ := env.GetConfigDir()
	userDataDir, _ := env.GetUserDataDir()
	hostname := env.GetHostname()

	runtime := map[string]any{
		"is_phone":      env.IsPhone(),
		"hostname":      hostname,
		"home":          home,
		"dotfiles_root": dotfilesRoot,
		"config_dir":    configDir,
		"user_data_dir": userDataDir,
	}
	if len(resolvedInputs) > 0 {
		runtime["inputs"] = resolvedInputs
	}

	payload := map[string]any{
		"workspaced": map[string]any{
			"runtime": runtime,
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
			return nil, fmt.Errorf("resolve runtime input %q: invalid from %q", name, spec)
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
