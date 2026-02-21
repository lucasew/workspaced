package modfile

import (
	"path/filepath"
	"testing"
)

func TestWorkspacePaths(t *testing.T) {
	t.Parallel()

	root := filepath.Clean(t.TempDir())
	ws := NewWorkspace(root)
	if ws.Root != root {
		t.Fatalf("root mismatch: got=%q want=%q", ws.Root, root)
	}

	if ws.ModPath() != filepath.Join(root, "workspaced.mod.toml") {
		t.Fatalf("mod path mismatch: got=%q", ws.ModPath())
	}
	if ws.SumPath() != filepath.Join(root, "workspaced.sum.toml") {
		t.Fatalf("sum path mismatch: got=%q", ws.SumPath())
	}
	if ws.ModulesBaseDir() != filepath.Join(root, "modules") {
		t.Fatalf("modules dir mismatch: got=%q", ws.ModulesBaseDir())
	}
}
