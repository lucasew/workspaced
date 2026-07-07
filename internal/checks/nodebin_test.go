package checks_test

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"workspaced/internal/checks"
)

func TestNodeModuleBinRel(t *testing.T) {
	t.Parallel()
	got := checks.NodeModuleBinRel("prettier")
	want := filepath.Join("node_modules", ".bin", "prettier")
	if got != want {
		t.Fatalf("NodeModuleBinRel: got %q want %q", got, want)
	}
}

func TestRequireNodeModuleBin(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	err := checks.RequireNodeModuleBin(dir, "eslint")
	if !errors.Is(err, checks.ErrNotApplicable) {
		t.Fatalf("missing bin: got %v want ErrNotApplicable", err)
	}

	binDir := filepath.Join(dir, "node_modules", ".bin")
	if err := os.MkdirAll(binDir, 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(binDir, "eslint"), []byte("#!/bin/sh\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := checks.RequireNodeModuleBin(dir, "eslint"); err != nil {
		t.Fatalf("present bin: %v", err)
	}
}
