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
	// sources top-level is no longer processed (leftovers removed); load succeeds with empty deps.
	if err != nil {
		t.Fatalf("unexpected error on legacy sources shape: %v", err)
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
	// sources top-level is no longer processed (leftovers removed).
	if err != nil {
		t.Fatalf("unexpected error on legacy sources shape: %v", err)
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

func TestGenericSourceLockFallbackWithoutProvider(t *testing.T) {
	t.Parallel()

	locked := LockedSource{Ref: "v1", Hash: "abc"}
	if !sourceLockReusable(locked) {
		t.Fatal("generic reusable requires hash only")
	}
	if !sourceLockMatchesDesired(LockedSource{}, locked) {
		t.Fatal("empty desired matches")
	}
	if !sourceLockMatchesDesired(LockedSource{Ref: "v1"}, locked) {
		t.Fatal("same ref matches")
	}
	if sourceLockMatchesDesired(LockedSource{Ref: "v2"}, locked) {
		t.Fatal("different ref must not match")
	}
}

func TestLoadSumFileToolLockUsesCurrentValueOverVersion(t *testing.T) {
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
	lock, ok := got.Tool("github:burntsushi/ripgrep")
	if !ok {
		t.Fatalf("missing ripgrep lock")
	}
	if lock.Version != "14.1.1" {
		t.Fatalf("version mismatch: got=%q", lock.Version)
	}
}
