package mod

import (
	"os"
	"path/filepath"
	"testing"
)

func TestFindRepoRootFromModFile(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "workspaced.mod.toml"), []byte("[modules]\n"), 0644); err != nil {
		t.Fatalf("write mod: %v", err)
	}
	child := filepath.Join(root, "workspaced", "cmd")
	if err := os.MkdirAll(child, 0755); err != nil {
		t.Fatalf("mkdir child: %v", err)
	}

	got, ok := findRepoRootFrom(child)
	if !ok {
		t.Fatal("expected root detection by workspaced.mod.toml")
	}
	if got != root {
		t.Fatalf("root mismatch: got=%q want=%q", got, root)
	}
}

func TestFindRepoRootFromSettingsAndModules(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "settings.toml"), []byte(""), 0644); err != nil {
		t.Fatalf("write settings: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(root, "modules"), 0755); err != nil {
		t.Fatalf("mkdir modules: %v", err)
	}
	child := filepath.Join(root, "nested", "dir")
	if err := os.MkdirAll(child, 0755); err != nil {
		t.Fatalf("mkdir child: %v", err)
	}

	got, ok := findRepoRootFrom(child)
	if !ok {
		t.Fatal("expected root detection by settings/modules markers")
	}
	if got != root {
		t.Fatalf("root mismatch: got=%q want=%q", got, root)
	}
}
