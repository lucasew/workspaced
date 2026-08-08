package deployer

import (
	"log/slog"
	"os"
	"path/filepath"
	"testing"

	"github.com/lucasew/workspaced/internal/cmdctx"
	"github.com/lucasew/workspaced/internal/source"
	"github.com/lucasew/workspaced/pkg/logging"
	"github.com/lucasew/workspaced/pkg/taskgroup"
)

func TestPlannerDetectsCommentOnlyContentChange(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "rg")
	if err := os.WriteFile(target, []byte("#!/usr/bin/env bash\n# locked: old\nexec true\n"), 0o755); err != nil {
		t.Fatalf("write target: %v", err)
	}

	desired := []DesiredState{{
		File: &source.BufferFile{
			BasicFile: source.BasicFile{
				RelPathStr:    "rg",
				TargetBaseDir: dir,
				FileMode:      0o755,
				Info:          "source:config-tree (.local/bin/_index.tmpl) (multi:rg)",
				FileType:      source.TypeMultiFile,
			},
			Content: []byte("#!/usr/bin/env bash\n# locked: new\nexec true\n"),
		},
	}}
	state := &State{Files: map[string]ManagedInfo{
		target: {SourceInfo: "source:config-tree (.local/bin/_index.tmpl) (multi:rg)"},
	}}

	g, ctx := taskgroup.New(logging.ContextWithLogger(t.Context(), slog.Default()), taskgroup.DefaultLimits())
	_ = g
	actions, err := NewPlanner().Plan(ctx, desired, state)
	if err != nil {
		t.Fatalf("plan: %v", err)
	}
	if len(actions) != 1 {
		t.Fatalf("actions count mismatch: got=%d", len(actions))
	}
	if actions[0].Type != ActionUpdate {
		t.Fatalf("action mismatch: got=%s", actions[0].Type)
	}
}

func TestPlannerNoCacheForcesUpdateOnIdenticalContent(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "same")
	content := []byte("unchanged\n")
	if err := os.WriteFile(target, content, 0o644); err != nil {
		t.Fatalf("write target: %v", err)
	}

	srcInfo := "module:x bundle:abc (same)"
	desired := []DesiredState{{
		File: &source.BufferFile{
			BasicFile: source.BasicFile{
				RelPathStr:    "same",
				TargetBaseDir: dir,
				FileMode:      0o644,
				Info:          srcInfo,
				FileType:      source.TypeStatic,
			},
			Content: content,
		},
	}}
	state := &State{Files: map[string]ManagedInfo{
		target: {SourceInfo: srcInfo},
	}}

	base := logging.ContextWithLogger(t.Context(), slog.Default())

	// Warm path: identical managed bundle → noop
	// (taskgroup tasks use Group's root ctx; set flags before New.)
	g, ctx := taskgroup.New(base, taskgroup.DefaultLimits())
	_ = g
	actions, err := NewPlanner().Plan(ctx, desired, state)
	if err != nil {
		t.Fatalf("plan warm: %v", err)
	}
	if len(actions) != 1 || actions[0].Type != ActionNoop {
		t.Fatalf("warm want noop, got %#v", actions)
	}

	// no-cache: same inputs → update
	g, ctx = taskgroup.New(cmdctx.WithNoCache(base, true), taskgroup.DefaultLimits())
	_ = g
	actions, err = NewPlanner().Plan(ctx, desired, state)
	if err != nil {
		t.Fatalf("plan no-cache: %v", err)
	}
	if len(actions) != 1 || actions[0].Type != ActionUpdate {
		t.Fatalf("no-cache want update, got %#v", actions)
	}
}

func TestPlannerIgnoredEqualIsNoop(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "placed.md")
	content := []byte("hello\n")
	if err := os.WriteFile(target, content, 0o644); err != nil {
		t.Fatal(err)
	}
	desired := []DesiredState{{
		File: &source.BufferFile{
			BasicFile: source.BasicFile{
				RelPathStr:    "placed.md",
				TargetBaseDir: dir,
				FileMode:      0o644,
				Info:          "module:place (placed.md)",
				FileType:      source.TypeStatic,
			},
			Content: content,
		},
	}}
	state := &State{Files: map[string]ManagedInfo{}}
	p := NewPlanner()
	p.Ignore = func(path string) bool { return path == target }

	g, ctx := taskgroup.New(logging.ContextWithLogger(t.Context(), slog.Default()), taskgroup.DefaultLimits())
	_ = g
	actions, err := p.Plan(ctx, desired, state)
	if err != nil {
		t.Fatal(err)
	}
	if len(actions) != 1 || actions[0].Type != ActionNoop {
		t.Fatalf("want noop, got %#v", actions)
	}
}

func TestPlannerIgnoredMissingIsCreate(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "placed.md")
	desired := []DesiredState{{
		File: &source.BufferFile{
			BasicFile: source.BasicFile{
				RelPathStr:    "placed.md",
				TargetBaseDir: dir,
				FileMode:      0o644,
				Info:          "module:place (placed.md)",
				FileType:      source.TypeStatic,
			},
			Content: []byte("hello\n"),
		},
	}}
	p := NewPlanner()
	p.Ignore = func(path string) bool { return path == target }

	g, ctx := taskgroup.New(logging.ContextWithLogger(t.Context(), slog.Default()), taskgroup.DefaultLimits())
	_ = g
	actions, err := p.Plan(ctx, desired, &State{Files: map[string]ManagedInfo{}})
	if err != nil {
		t.Fatal(err)
	}
	if len(actions) != 1 || actions[0].Type != ActionCreate {
		t.Fatalf("want create, got %#v", actions)
	}
}

func TestPlannerUnmanagedEqualStillAdopts(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "tracked.md")
	content := []byte("hello\n")
	if err := os.WriteFile(target, content, 0o644); err != nil {
		t.Fatal(err)
	}
	desired := []DesiredState{{
		File: &source.BufferFile{
			BasicFile: source.BasicFile{
				RelPathStr:    "tracked.md",
				TargetBaseDir: dir,
				FileMode:      0o644,
				Info:          "module:x (tracked.md)",
				FileType:      source.TypeStatic,
			},
			Content: content,
		},
	}}
	g, ctx := taskgroup.New(logging.ContextWithLogger(t.Context(), slog.Default()), taskgroup.DefaultLimits())
	_ = g
	actions, err := NewPlanner().Plan(ctx, desired, &State{Files: map[string]ManagedInfo{}})
	if err != nil {
		t.Fatal(err)
	}
	if len(actions) != 1 || actions[0].Type != ActionUpdate {
		t.Fatalf("want adopt update, got %#v", actions)
	}
}
