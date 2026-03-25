package configcue

import (
	"embed"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"

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
	Cwd string
}

type DiscoverResult struct {
	Layers []Layer `json:"layers"`
}

func DiscoverLayers(opts DiscoverOptions) (DiscoverResult, error) {
	layers := make([]Layer, 0)

	repoPath, err := findUp(opts.Cwd, "workspaced.cue")
	if err != nil {
		return DiscoverResult{}, err
	}
	if repoPath != "" {
		layers = append(layers, Layer{Name: "repo", Path: repoPath})
	}

	dotfilesRoot, err := env.GetDotfilesRoot()
	if err == nil && dotfilesRoot != "" {
		p := filepath.Join(dotfilesRoot, "workspaced.cue")
		if fileExists(p) {
			layers = append(layers, Layer{Name: "dotfiles", Path: p})
		}
	}

	configDir, err := env.GetConfigDir()
	if err == nil && configDir != "" {
		p := filepath.Join(configDir, "workspaced.cue")
		if fileExists(p) {
			layers = append(layers, Layer{Name: "home", Path: p})
		}
	}

	return DiscoverResult{Layers: layers}, nil
}

func ExportJSON(opts DiscoverOptions) ([]byte, error) {
	discovered, err := DiscoverLayers(opts)
	if err != nil {
		return nil, err
	}
	paths := make([]string, 0, len(discovered.Layers))
	for _, layer := range discovered.Layers {
		paths = append(paths, layer.Path)
	}
	return ExportJSONFromPaths(paths)
}

func ExportJSONFromPaths(paths []string) ([]byte, error) {
	return exportJSONFromPaths(paths, nil)
}

func exportJSONFromPaths(paths []string, discovered []Layer) ([]byte, error) {
	ctx := cuecontext.New()
	schemaBytes, err := schemaFS.ReadFile("schema.cue")
	if err != nil {
		return nil, fmt.Errorf("read embedded cue schema: %w", err)
	}
	preludeBytes, err := schemaFS.ReadFile("prelude.cue")
	if err != nil {
		return nil, fmt.Errorf("read embedded cue prelude: %w", err)
	}

	v := ctx.CompileString(string(schemaBytes), cue.Filename("schema.cue"))
	if err := v.Err(); err != nil {
		return nil, fmt.Errorf("compile embedded cue schema: %w", err)
	}

	preludeLayer := ctx.CompileString(string(preludeBytes), cue.Filename("prelude.cue"))
	if err := preludeLayer.Err(); err != nil {
		return nil, fmt.Errorf("compile embedded cue prelude: %w", err)
	}
	v = v.Unify(preludeLayer)
	if err := v.Err(); err != nil {
		return nil, fmt.Errorf("unify embedded cue prelude: %w", err)
	}

	runtimePrelude, err := buildRuntimePrelude()
	if err != nil {
		return nil, err
	}
	runtimeLayer := ctx.CompileString(runtimePrelude, cue.Filename("runtime_prelude.cue"))
	if err := runtimeLayer.Err(); err != nil {
		return nil, fmt.Errorf("compile runtime cue prelude: %w", err)
	}
	v = v.Unify(runtimeLayer)
	if err := v.Err(); err != nil {
		return nil, fmt.Errorf("unify runtime cue prelude: %w", err)
	}

	for _, path := range paths {
		src, err := os.ReadFile(path)
		if err != nil {
			return nil, fmt.Errorf("read cue layer %s: %w", path, err)
		}
		layerValue := ctx.CompileString(string(src), cue.Filename(path))
		if err := layerValue.Err(); err != nil {
			return nil, fmt.Errorf("compile cue layer %s: %w", path, err)
		}
		v = v.Unify(layerValue)
		if err := v.Err(); err != nil {
			return nil, fmt.Errorf("unify cue layer %s: %w", path, err)
		}
	}

	configValue := v.LookupPath(cue.ParsePath("workspaced"))
	if err := configValue.Err(); err != nil {
		return nil, fmt.Errorf("lookup workspaced value: %w", err)
	}
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

func fileExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}

func buildRuntimePrelude() (string, error) {
	home, _ := os.UserHomeDir()
	dotfilesRoot, _ := env.GetDotfilesRoot()
	configDir, _ := env.GetConfigDir()
	userDataDir, _ := env.GetUserDataDir()
	hostname := env.GetHostname()

	runtime := map[string]any{
		"is_phone":     env.IsPhone(),
		"hostname":     hostname,
		"home":         home,
		"dotfiles_root": dotfilesRoot,
		"config_dir":   configDir,
		"user_data_dir": userDataDir,
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
