package env_test

import (
	"os"
	"path/filepath"
	"testing"

	"workspaced/pkg/driver/env"
)

func TestExpandPathInUsesProvidedHome(t *testing.T) {
	t.Parallel()
	got := env.ExpandPathIn("~/.config/workspaced", "/data/home")
	want := filepath.Join("/data/home", ".config/workspaced")
	if got != want {
		t.Fatalf("ExpandPathIn = %q, want %q", got, want)
	}
}

func TestFindDotfilesRoot(t *testing.T) {
	home := t.TempDir()
	root := filepath.Join(home, ".dotfiles")
	if err := os.Mkdir(root, 0o755); err != nil {
		t.Fatal(err)
	}
	got, err := env.FindDotfilesRoot(home)
	if err != nil {
		t.Fatal(err)
	}
	if got != root {
		t.Fatalf("FindDotfilesRoot = %q, want %q", got, root)
	}
}

func TestEnsureUnderHome(t *testing.T) {
	home := t.TempDir()
	got, err := env.EnsureUnderHome(home, ".local/share/workspaced")
	if err != nil {
		t.Fatal(err)
	}
	want := filepath.Join(home, ".local/share/workspaced")
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
	if _, err := os.Stat(got); err != nil {
		t.Fatal(err)
	}
}
