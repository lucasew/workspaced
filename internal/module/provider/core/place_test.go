package core

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/lucasew/workspaced/internal/module"
	"github.com/lucasew/workspaced/pkg/logging"
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
		if !errors.Is(err, os.ErrNotExist) {
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
		if len(out.Files) != 1 {
			t.Fatalf("files=%d want 1: %+v", len(out.Files), out.Files)
		}
		if out.Files[0].RelPath != "out-ok/exists.txt" {
			t.Fatalf("RelPath=%q want out-ok/exists.txt", out.Files[0].RelPath)
		}
		if out.Files[0].AbsPath != existing {
			t.Fatalf("AbsPath=%q want %q", out.Files[0].AbsPath, existing)
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
		if len(out.Files) != 0 {
			t.Fatalf("files=%d want 0: %+v", len(out.Files), out.Files)
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
	if len(out.Files) != 2 {
		t.Fatalf("files=%d want 2: %+v", len(out.Files), out.Files)
	}
	// sorted by RelPath
	if out.Files[0].RelPath != ".config/app/a.txt" {
		t.Fatalf("first=%q", out.Files[0].RelPath)
	}
	if out.Files[1].RelPath != ".config/app/sub/b.txt" {
		t.Fatalf("second=%q", out.Files[1].RelPath)
	}
}

func TestPlaceStepsMoveAndRequire(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	pkg := filepath.Join(root, "skill")
	if err := os.MkdirAll(pkg, 0o755); err != nil {
		t.Fatal(err)
	}
	skillPath := filepath.Join(pkg, "SKILL.md")
	if err := os.WriteFile(skillPath, []byte("# skill"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(pkg, "notes.md"), []byte("n"), 0o644); err != nil {
		t.Fatal(err)
	}

	ctx := logging.NewWriterContext(t.Output())
	out, err := placeModule{}.Resolve(ctx, module.ResolveRequest{
		ModuleName: "best_practices",
		ModuleConfig: map[string]any{
			"items": map[string]any{
				"skills/best-practices/references/go": pkg,
			},
			"steps": map[string]any{
				"10_require": map[string]any{
					"op": "require",
					"patterns": map[string]any{
						"skill": "SKILL.md",
					},
				},
				"20_demote": map[string]any{
					"op":   "move",
					"from": "SKILL.md",
					"to":   "entry.md",
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	rels := map[string]string{}
	for _, f := range out.Files {
		rels[f.RelPath] = f.AbsPath
	}
	wantEntry := "skills/best-practices/references/go/entry.md"
	if rels[wantEntry] != skillPath {
		t.Fatalf("entry path: got %q want abs %q; files=%v", rels[wantEntry], skillPath, rels)
	}
	if _, ok := rels["skills/best-practices/references/go/SKILL.md"]; ok {
		t.Fatal("SKILL.md should have been moved")
	}
	if _, ok := rels["skills/best-practices/references/go/notes.md"]; !ok {
		t.Fatal("notes.md missing")
	}
}

func TestPlaceRequireFailsWhenMissing(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	pkg := filepath.Join(root, "skill")
	if err := os.MkdirAll(pkg, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(pkg, "README.md"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}

	ctx := logging.NewWriterContext(t.Output())
	_, err := placeModule{}.Resolve(ctx, module.ResolveRequest{
		ModuleName: "best_practices",
		ModuleConfig: map[string]any{
			"items": map[string]any{"out": pkg},
			"steps": map[string]any{
				"10_require": map[string]any{
					"op":       "require",
					"patterns": map[string]any{"skill": "SKILL.md"},
				},
			},
		},
	})
	if err == nil {
		t.Fatal("expected require failure")
	}
	if !errors.Is(err, errPlaceRequireNoMatch) {
		t.Fatalf("error=%v want %v", err, errPlaceRequireNoMatch)
	}
}

func TestPlaceRequireNegation(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	pkg := filepath.Join(root, "skill")
	if err := os.MkdirAll(pkg, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(pkg, "SKILL.md"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(pkg, "entry.md"), []byte("y"), 0o644); err != nil {
		t.Fatal(err)
	}

	ctx := logging.NewWriterContext(t.Output())
	_, err := placeModule{}.Resolve(ctx, module.ResolveRequest{
		ModuleName: "best_practices",
		ModuleConfig: map[string]any{
			"items": map[string]any{"out": pkg},
			"steps": map[string]any{
				"10_require": map[string]any{
					"op": "require",
					"patterns": map[string]any{
						"skill":    "SKILL.md",
						"no_entry": "!entry.md",
					},
				},
			},
		},
	})
	if err == nil {
		t.Fatal("expected negation failure")
	}
	if !errors.Is(err, errPlaceRequireMustNot) {
		t.Fatalf("error=%v want %v", err, errPlaceRequireMustNot)
	}
}

func TestPlaceMoveMissingWarns(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	pkg := filepath.Join(root, "skill")
	if err := os.MkdirAll(pkg, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(pkg, "a.md"), []byte("a"), 0o644); err != nil {
		t.Fatal(err)
	}

	ctx := logging.NewWriterContext(t.Output())
	out, err := placeModule{}.Resolve(ctx, module.ResolveRequest{
		ModuleName: "m",
		ModuleConfig: map[string]any{
			"items": map[string]any{"out": pkg},
			"steps": map[string]any{
				"20_demote": map[string]any{
					"op":   "move",
					"from": "SKILL.md",
					"to":   "entry.md",
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if len(out.Warnings) != 1 {
		t.Fatalf("warnings=%v", out.Warnings)
	}
	if !strings.Contains(out.Warnings[0], "missing") {
		t.Fatalf("warning=%q", out.Warnings[0])
	}
	if len(out.Files) != 1 || !strings.HasSuffix(out.Files[0].RelPath, "a.md") {
		t.Fatalf("files=%+v", out.Files)
	}
}

func TestPlaceMoveOverwriteWarns(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	pkg := filepath.Join(root, "skill")
	if err := os.MkdirAll(pkg, 0o755); err != nil {
		t.Fatal(err)
	}
	skill := filepath.Join(pkg, "SKILL.md")
	entry := filepath.Join(pkg, "entry.md")
	if err := os.WriteFile(skill, []byte("skill"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(entry, []byte("old"), 0o644); err != nil {
		t.Fatal(err)
	}

	ctx := logging.NewWriterContext(t.Output())
	out, err := placeModule{}.Resolve(ctx, module.ResolveRequest{
		ModuleName: "m",
		ModuleConfig: map[string]any{
			"items": map[string]any{"out": pkg},
			"steps": map[string]any{
				"20_demote": map[string]any{
					"op":   "move",
					"from": "SKILL.md",
					"to":   "entry.md",
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if len(out.Warnings) != 1 || !strings.Contains(out.Warnings[0], "overwrites") {
		t.Fatalf("warnings=%v", out.Warnings)
	}
	if len(out.Files) != 1 {
		t.Fatalf("files=%+v want 1 (overwrite)", out.Files)
	}
	if out.Files[0].AbsPath != skill {
		t.Fatalf("AbsPath=%q want skill content path", out.Files[0].AbsPath)
	}
}

func TestPlaceMoveDirPrefix(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	pkg := filepath.Join(root, "skill")
	if err := os.MkdirAll(filepath.Join(pkg, "docs"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(pkg, "docs", "a.md"), []byte("a"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(pkg, "keep.md"), []byte("k"), 0o644); err != nil {
		t.Fatal(err)
	}

	ctx := logging.NewWriterContext(t.Output())
	out, err := placeModule{}.Resolve(ctx, module.ResolveRequest{
		ModuleName: "m",
		ModuleConfig: map[string]any{
			"items": map[string]any{"out": pkg},
			"steps": map[string]any{
				"10_mv": map[string]any{
					"op":   "move",
					"from": "docs",
					"to":   "reference",
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	rels := map[string]bool{}
	for _, f := range out.Files {
		rels[f.RelPath] = true
	}
	if !rels["out/reference/a.md"] {
		t.Fatalf("expected dir prefix move: %v", rels)
	}
	if rels["out/docs/a.md"] {
		t.Fatalf("docs should be gone: %v", rels)
	}
	if !rels["out/keep.md"] {
		t.Fatalf("keep missing: %v", rels)
	}
}

func TestPlaceUnknownOp(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "a"), []byte("a"), 0o644); err != nil {
		t.Fatal(err)
	}
	ctx := logging.NewWriterContext(t.Output())
	_, err := placeModule{}.Resolve(ctx, module.ResolveRequest{
		ModuleName: "m",
		ModuleConfig: map[string]any{
			"items": map[string]any{"out": root},
			"steps": map[string]any{
				"x": map[string]any{"op": "reject"},
			},
		},
	})
	if err == nil || !errors.Is(err, errPlaceUnknownOp) {
		t.Fatalf("error=%v want %v", err, errPlaceUnknownOp)
	}
}

func TestCleanPlacePath(t *testing.T) {
	t.Parallel()
	if _, err := cleanPlacePath("../x"); !errors.Is(err, errPlacePathEscape) {
		t.Fatalf("escape: %v", err)
	}
	if _, err := cleanPlacePath(""); !errors.Is(err, errPlaceEmptyPath) {
		t.Fatalf("empty: %v", err)
	}
	got, err := cleanPlacePath("foo/bar")
	if err != nil || got != "foo/bar" {
		t.Fatalf("got %q err %v", got, err)
	}
}
