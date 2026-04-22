package modfile

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestLoadSumFileRequiresSource(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	sumPath := filepath.Join(dir, "workspaced.lock.json")
	content, _ := json.Marshal(map[string]any{
		"modules": map[string]any{
			"foo": map[string]any{"version": "v1.0.0"},
		},
	})
	if err := os.WriteFile(sumPath, content, 0644); err != nil {
		t.Fatalf("write sum: %v", err)
	}

	got, err := LoadSumFile(sumPath)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got.Dependencies) != 0 {
		t.Fatalf("expected empty dependencies, got=%d", len(got.Dependencies))
	}
}

func TestLoadSumFileRequiresSourceProvider(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	sumPath := filepath.Join(dir, "workspaced.lock.json")
	content, _ := json.Marshal(map[string]any{
		"sources": map[string]any{
			"papirus": map[string]any{"path": "/tmp/papirus"},
		},
	})
	if err := os.WriteFile(sumPath, content, 0644); err != nil {
		t.Fatalf("write sum: %v", err)
	}

	_, err := LoadSumFile(sumPath)
	if err == nil {
		t.Fatal("expected provider required error")
	}
}

func TestLoadSumFileRequiresSourceHash(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	sumPath := filepath.Join(dir, "workspaced.lock.json")
	content, _ := json.Marshal(map[string]any{
		"sources": map[string]any{
			"papirus": map[string]any{
				"provider": "github",
				"repo":     "PapirusDevelopmentTeam/papirus-icon-theme",
				"url":      "https://codeload.github.com/PapirusDevelopmentTeam/papirus-icon-theme/tar.gz/main",
			},
		},
	})
	if err := os.WriteFile(sumPath, content, 0644); err != nil {
		t.Fatalf("write sum: %v", err)
	}

	_, err := LoadSumFile(sumPath)
	if err == nil {
		t.Fatal("expected hash required error")
	}
}

func TestLoadSumFileMissingIsEmpty(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	sumPath := filepath.Join(dir, "missing.lock.json")
	got, err := LoadSumFile(sumPath)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got.Dependencies) != 0 {
		t.Fatalf("expected empty dependencies, got=%d", len(got.Dependencies))
	}
}

func TestLoadSumFileToolLockUsesVersionOverCurrentValue(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	sumPath := filepath.Join(dir, "workspaced.lock.json")
	if err := os.WriteFile(sumPath, []byte(`{
  "dependencies": [
    {
      "kind": "tool",
      "name": "ripgrep",
      "ref": "github:burntsushi/ripgrep",
      "version": "15.1.0",
      "currentValue": "14.1.1"
    }
  ]
}
`), 0644); err != nil {
		t.Fatalf("write sum: %v", err)
	}

	got, err := LoadSumFile(sumPath)
	if err != nil {
		t.Fatalf("load sum: %v", err)
	}
	lock, ok := got.Tool("ripgrep")
	if !ok {
		t.Fatalf("missing ripgrep lock")
	}
	if lock.Version != "15.1.0" {
		t.Fatalf("version mismatch: got=%q", lock.Version)
	}
}
