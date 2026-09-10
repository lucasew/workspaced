package configcue

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	_ "github.com/lucasew/workspaced/pkg/driver/env/native"
	"github.com/lucasew/workspaced/pkg/filespine"
	"github.com/lucasew/workspaced/pkg/logging"
)

func TestRuntimeModeAndFileMap(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "workspaced.cue"), `package workspaced
workspaced: file: {
	".codex/config.toml": {type: "toml", values: {model: "x"}}
	codebase: {
		".gitignore": {type: "text", values: {content: "bin/"}}
	}
}
`)

	ctx := logging.NewWriterContext(t.Output())
	cuePath := filepath.Join(root, "workspaced.cue")
	home, err := loadFilesMode(ctx, cuePath, filespine.ModeHome)
	if err != nil {
		t.Fatalf("load home: %v", err)
	}
	if got := home.RuntimeMode(); got != filespine.ModeHome {
		t.Fatalf("home mode=%q", got)
	}
	homeFiles, err := home.FileMap()
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := homeFiles[".codex/config.toml"]; !ok {
		t.Fatalf("home FileMap keys=%v want .codex/config.toml", keysOf(homeFiles))
	}
	if _, ok := homeFiles[".gitignore"]; ok {
		t.Fatalf("home FileMap leaked codebase dest: %v", keysOf(homeFiles))
	}

	code, err := loadFilesMode(ctx, cuePath, filespine.ModeCodebase)
	if err != nil {
		t.Fatalf("load codebase: %v", err)
	}
	if got := code.RuntimeMode(); got != filespine.ModeCodebase {
		t.Fatalf("codebase mode=%q", got)
	}
	codeFiles, err := code.FileMap()
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := codeFiles[".gitignore"]; !ok {
		t.Fatalf("codebase FileMap keys=%v want .gitignore", keysOf(codeFiles))
	}
	if _, ok := codeFiles[".codex/config.toml"]; ok {
		t.Fatalf("codebase FileMap leaked home dest: %v", keysOf(codeFiles))
	}
}

func loadFilesMode(ctx context.Context, path, mode string) (*Config, error) {
	v, err := buildWorkspacedValue(ctx, []string{path}, nil, DiscoverOptions{Mode: mode})
	if err != nil {
		return nil, err
	}
	data, err := marshalWorkspacedValue(ctx, v, []string{path}, nil)
	if err != nil {
		return nil, err
	}
	cfg, err := decodeConfig(data)
	if err != nil {
		return nil, err
	}
	cfg.cueVal = v
	return cfg, nil
}

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}
