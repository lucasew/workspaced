package source

import (
	"os"
	"path/filepath"
	"testing"

	"workspaced/internal/configcue"
	_ "workspaced/pkg/driver/env/native"
	"workspaced/pkg/logging"
	_ "workspaced/internal/module/prelude"
	"workspaced/pkg/taskgroup"
)

func TestCloneModuleConfigIsolatesNestedMaps(t *testing.T) {
	t.Parallel()
	orig := map[string]any{
		"items": map[string]any{"a": "src:a", "b": "src:b"},
		"tags":  []any{"x", map[string]any{"k": "v"}},
	}
	cloned := cloneModuleConfig(orig)
	items := cloned["items"].(map[string]any)
	items["a"] = "mutated"
	tags := cloned["tags"].([]any)
	tags[1].(map[string]any)["k"] = "mutated"

	origItems := orig["items"].(map[string]any)
	if origItems["a"] != "src:a" {
		t.Fatalf("original nested map mutated: %v", origItems["a"])
	}
	origTags := orig["tags"].([]any)
	if origTags[1].(map[string]any)["k"] != "v" {
		t.Fatalf("original nested slice map mutated: %v", origTags[1])
	}
}

func TestCloneModuleConfigNilBecomesEmpty(t *testing.T) {
	t.Parallel()
	got := cloneModuleConfig(nil)
	if got == nil || len(got) != 0 {
		t.Fatalf("got %#v", got)
	}
	got["x"] = 1
}

func TestModuleScannerProcessMapReduceOrder(t *testing.T) {
	root := t.TempDir()
	modulesDir := filepath.Join(root, "modules")
	if err := os.MkdirAll(modulesDir, 0o755); err != nil {
		t.Fatal(err)
	}
	srcA := filepath.Join(root, "src-a")
	srcB := filepath.Join(root, "src-b")
	for _, dir := range []string{srcA, srcB} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	writeFile(t, filepath.Join(srcA, "a.txt"), "a")
	writeFile(t, filepath.Join(srcB, "b.txt"), "b")

	writeFile(t, filepath.Join(root, "workspaced.cue"), `package workspaced

workspaced: {
	modules: {
		zebra: {
			enable: true
			from: "core:place"
			config: {
				items: {
					"out-z": "`+srcB+`"
				}
			}
		}
		alpha: {
			enable: true
			from: "core:place"
			config: {
				items: {
					"out-a": "`+srcA+`"
				}
			}
		}
		noop: {
			enable: false
			from: "core:place"
		}
	}
}
`)
	writeFile(t, filepath.Join(root, "workspaced.lock.json"), `{"dependencies":[]}`)

	g, ctx := taskgroup.New(logging.NewWriterContext(t.Output()), taskgroup.DefaultLimits())
	t.Cleanup(func() { _ = g.Wait() })

	cfg, err := configcue.LoadForWorkspace(ctx, root)
	if err != nil {
		t.Fatalf("load config: %v", err)
	}

	plugin := NewModuleScannerPlugin(modulesDir, cfg, 100)
	out, err := plugin.Process(ctx, nil)
	if err != nil {
		t.Fatalf("Process: %v", err)
	}
	if len(out) != 2 {
		t.Fatalf("files=%d want 2: %#v", len(out), relPaths(out))
	}
	// Enabled modules are sorted by name: alpha then zebra.
	if got := moduleName(out[0]); got != "alpha" {
		t.Fatalf("first module=%q want alpha", got)
	}
	if got := moduleName(out[1]); got != "zebra" {
		t.Fatalf("second module=%q want zebra", got)
	}
	if out[0].RelPath() != "out-a/a.txt" {
		t.Fatalf("alpha path=%q want out-a/a.txt", out[0].RelPath())
	}
	if out[1].RelPath() != "out-z/b.txt" {
		t.Fatalf("zebra path=%q want out-z/b.txt", out[1].RelPath())
	}
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

func moduleName(f File) string {
	if sf, ok := f.(ScopedFile); ok {
		return sf.ModuleName()
	}
	return ""
}

func relPaths(files []File) []string {
	out := make([]string, len(files))
	for i, f := range files {
		out[i] = f.RelPath()
	}
	return out
}
