package core

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"workspaced/internal/module"
	"workspaced/pkg/logging"
)

func TestPlaceResolveIgnoreMissing(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	existing := filepath.Join(root, "exists.txt")
	if err := os.WriteFile(existing, []byte("hi"), 0o644); err != nil {
		t.Fatal(err)
	}
	missing := filepath.Join(root, "nope.txt")

	ctx := logging.NewWriterContext(t.Output())
	m := placeModule{}

	t.Run("default fails on missing", func(t *testing.T) {
		t.Parallel()
		_, err := m.Resolve(ctx, module.ResolveRequest{
			ModuleName: "test-place",
			ModuleConfig: map[string]any{
				"items": map[string]any{
					"out": missing,
				},
			},
		})
		if err == nil {
			t.Fatal("expected error for missing source")
		}
		if !strings.Contains(err.Error(), "place source") {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	t.Run("ignore_missing skips missing keeps present", func(t *testing.T) {
		t.Parallel()
		out, err := m.Resolve(ctx, module.ResolveRequest{
			ModuleName: "test-place",
			ModuleConfig: map[string]any{
				"ignore_missing": true,
				"items": map[string]any{
					"out-missing": missing,
					"out-ok":      existing,
				},
			},
		})
		if err != nil {
			t.Fatalf("Resolve: %v", err)
		}
		if len(out) != 1 {
			t.Fatalf("files=%d want 1: %+v", len(out), out)
		}
		if out[0].RelPath != "out-ok/exists.txt" {
			t.Fatalf("RelPath=%q want out-ok/exists.txt", out[0].RelPath)
		}
		if out[0].AbsPath != existing {
			t.Fatalf("AbsPath=%q want %q", out[0].AbsPath, existing)
		}
	})

	t.Run("ignore_missing all missing yields empty", func(t *testing.T) {
		t.Parallel()
		out, err := m.Resolve(ctx, module.ResolveRequest{
			ModuleName: "test-place",
			ModuleConfig: map[string]any{
				"ignore_missing": true,
				"items": map[string]any{
					"out": missing,
				},
			},
		})
		if err != nil {
			t.Fatalf("Resolve: %v", err)
		}
		if len(out) != 0 {
			t.Fatalf("files=%d want 0: %+v", len(out), out)
		}
	})
}

func TestPlaceResolveDirectory(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	dir := filepath.Join(root, "tree")
	if err := os.MkdirAll(filepath.Join(dir, "sub"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "a.txt"), []byte("a"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "sub", "b.txt"), []byte("b"), 0o644); err != nil {
		t.Fatal(err)
	}

	ctx := logging.NewWriterContext(t.Output())
	out, err := placeModule{}.Resolve(ctx, module.ResolveRequest{
		ModuleName: "test-place",
		ModuleConfig: map[string]any{
			"items": map[string]any{
				".config/app": dir,
			},
		},
	})
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if len(out) != 2 {
		t.Fatalf("files=%d want 2: %+v", len(out), out)
	}
	// sorted by RelPath
	if out[0].RelPath != ".config/app/a.txt" {
		t.Fatalf("first=%q", out[0].RelPath)
	}
	if out[1].RelPath != ".config/app/sub/b.txt" {
		t.Fatalf("second=%q", out[1].RelPath)
	}
}
