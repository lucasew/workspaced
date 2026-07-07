package checks_test

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"workspaced/internal/checks"
)

func TestRequireFile(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module x\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := checks.RequireFile(dir, "go.mod"); err != nil {
		t.Fatalf("present file: %v", err)
	}
	err := checks.RequireFile(dir, "missing.lock")
	if !errors.Is(err, checks.ErrNotApplicable) {
		t.Fatalf("missing file: got %v, want ErrNotApplicable", err)
	}
}
