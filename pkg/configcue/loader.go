package configcue

import (
	"embed"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"workspaced/pkg/env"

	"cuelang.org/go/cue"
	"cuelang.org/go/cue/cuecontext"
)

//go:embed schema.cue
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
	var layers []Layer

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

	ctx := cuecontext.New()
	schemaBytes, err := schemaFS.ReadFile("schema.cue")
	if err != nil {
		return nil, fmt.Errorf("read embedded cue schema: %w", err)
	}

	v := ctx.CompileString(string(schemaBytes), cue.Filename("schema.cue"))
	if err := v.Err(); err != nil {
		return nil, fmt.Errorf("compile embedded cue schema: %w", err)
	}

	for _, layer := range discovered.Layers {
		src, err := os.ReadFile(layer.Path)
		if err != nil {
			return nil, fmt.Errorf("read %s layer: %w", layer.Name, err)
		}
		layerValue := ctx.CompileString(string(src), cue.Filename(layer.Path))
		if err := layerValue.Err(); err != nil {
			return nil, fmt.Errorf("compile %s layer %s: %w", layer.Name, layer.Path, err)
		}
		v = v.Unify(layerValue)
		if err := v.Err(); err != nil {
			return nil, fmt.Errorf("unify %s layer %s: %w", layer.Name, layer.Path, err)
		}
	}

	configValue := v.LookupPath(cue.ParsePath("config"))
	if err := configValue.Err(); err != nil {
		return nil, fmt.Errorf("lookup config value: %w", err)
	}
	if !configValue.Exists() {
		return json.Marshal(map[string]any{})
	}
	b, err := configValue.MarshalJSON()
	if err != nil {
		return nil, fmt.Errorf("marshal cue config to json: %w", err)
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
