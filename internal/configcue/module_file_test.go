package configcue

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/lucasew/workspaced/internal/filespine"
	_ "github.com/lucasew/workspaced/pkg/driver/env/native"
	"github.com/lucasew/workspaced/pkg/logging"
)

func TestModuleFileLiftsIntoWorkspacedFile(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	modDir := filepath.Join(root, "modules", "greet")
	if err := os.MkdirAll(modDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(modDir, "module.cue"), []byte(`package module

module: {
	meta: {requires: [], recommends: []}
	config: {
		name: string | *"world"
	}
	file: {
		"hello.json": {
			type: "json"
			values: {
				ok:   true
				name: workspaced.modules.greet.config.name
			}
		}
	}
}
`), 0o644); err != nil {
		t.Fatal(err)
	}
	cuePath := filepath.Join(root, "workspaced.cue")
	if err := os.WriteFile(cuePath, []byte(`package workspaced
workspaced: {
	modules: greet: {
		input:  "self"
		path:   "modules/greet"
		enable: true
		config: {name: "ada"}
	}
}
`), 0o644); err != nil {
		t.Fatal(err)
	}
	ctx := logging.NewWriterContext(t.Output())
	cfg, err := LoadFiles(ctx, []string{cuePath})
	if err != nil {
		t.Fatal(err)
	}
	parsed, err := filespine.Parse(filespine.LookupFile(cfg.Cue()))
	if err != nil {
		t.Fatal(err)
	}
	got, ok := parsed["hello.json"]
	if !ok {
		t.Fatalf("file keys: %v", keysOf(parsed))
	}
	if got.Type != filespine.TypeJSON {
		t.Fatalf("type = %q", got.Type)
	}
	if got.Data["ok"] != true {
		t.Fatalf("ok = %#v", got.Data["ok"])
	}
	if got.Data["name"] != "ada" {
		t.Fatalf("name = %#v", got.Data["name"])
	}
}

func keysOf(m map[string]filespine.File) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}
