package module

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadSumFileRequiresSource(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	sumPath := filepath.Join(dir, "workspaced.sum.toml")
	content := `
[modules.foo]
version = "v1.0.0"
`
	if err := os.WriteFile(sumPath, []byte(content), 0644); err != nil {
		t.Fatalf("write sum: %v", err)
	}

	_, err := LoadSumFile(sumPath)
	if err == nil {
		t.Fatal("expected source required error")
	}
}

func TestLoadSumFileMissingIsEmpty(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	sumPath := filepath.Join(dir, "missing.sum.toml")
	got, err := LoadSumFile(sumPath)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got.Modules) != 0 {
		t.Fatalf("expected empty modules map, got=%d", len(got.Modules))
	}
}
