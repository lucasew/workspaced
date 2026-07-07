package types_test

import (
	"path/filepath"
	"testing"

	"workspaced/internal/types"
)

func TestDaemonSocketPathUsesXDGRuntimeDir(t *testing.T) {
	t.Setenv("XDG_RUNTIME_DIR", "/tmp/runtime-test")
	got := types.DaemonSocketPath()
	want := filepath.Join("/tmp/runtime-test", "workspaced.sock")
	if got != want {
		t.Fatalf("DaemonSocketPath() = %q, want %q", got, want)
	}
}

func TestDaemonSocketPathFallsBackWithoutXDG(t *testing.T) {
	t.Setenv("XDG_RUNTIME_DIR", "")
	got := types.DaemonSocketPath()
	if filepath.Base(got) != "workspaced.sock" {
		t.Fatalf("base = %q, want workspaced.sock", filepath.Base(got))
	}
	if filepath.Dir(got) == "" || filepath.Dir(got) == "." {
		t.Fatalf("unexpected dir in %q", got)
	}
}
