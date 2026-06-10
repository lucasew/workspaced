package deployer

import (
	"log/slog"
	"os"
	"path/filepath"
	"testing"

	"workspaced/pkg/logging"
	"workspaced/pkg/source"
	"workspaced/pkg/taskgroup"
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
				Info:          "source:legacy-config (.local/bin/_index.tmpl) (multi:rg)",
				FileType:      source.TypeMultiFile,
			},
			Content: []byte("#!/usr/bin/env bash\n# locked: new\nexec true\n"),
		},
	}}
	state := &State{Files: map[string]ManagedInfo{
		target: {SourceInfo: "source:legacy-config (.local/bin/_index.tmpl) (multi:rg)"},
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
