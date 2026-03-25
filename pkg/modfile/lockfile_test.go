package modfile

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"workspaced/pkg/config"
)

func TestBuildLockEntriesSkipsCoreAndLocal(t *testing.T) {
	t.Parallel()

	cfg := &config.GlobalConfig{
		Inputs: map[string]config.InputConfig{
			"remote": {From: "github:owner/repo", Version: "v1.0.0"},
		},
		Modules: map[string]any{
			"icons": map[string]any{"enable": true},
			"foo":   map[string]any{"enable": true, "input": "remote", "path": "path"},
			"bar":   map[string]any{"enable": true, "input": "self", "path": "modules/bar"},
		},
	}
	modFile := &ModFile{
		Sources: map[string]SourceConfig{},
	}

	got, err := BuildLockEntries(cfg, modFile, "/tmp/modules")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if _, ok := got["icons"]; ok {
		t.Fatalf("icons should not be lock entry for core provider")
	}
	if _, ok := got["bar"]; ok {
		t.Fatalf("bar should not be lock entry for self provider")
	}

	entry, ok := got["foo"]
	if !ok {
		t.Fatalf("expected foo lock entry")
	}
	if entry.Source != "github:owner/repo/path" {
		t.Fatalf("source mismatch: got=%q", entry.Source)
	}
	if entry.Version != "v1.0.0" {
		t.Fatalf("version mismatch: got=%q", entry.Version)
	}
}

func TestWriteSumFile(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	path := filepath.Join(dir, "workspaced.lock.json")

	err := WriteSumFile(path, &SumFile{
		Sources: map[string]LockedSource{
			"papirus": {Provider: "github", Repo: "PapirusDevelopmentTeam/papirus-icon-theme", URL: "https://codeload.github.com/PapirusDevelopmentTeam/papirus-icon-theme/tar.gz/main", Hash: "abc123"},
		},
		Modules: map[string]LockedModule{
			"zeta": {Source: "github:acme/zeta"},
			"alfa": {Source: "github:acme/alfa", Version: "v2"},
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read file: %v", err)
	}
	var got SumFile
	if err := json.Unmarshal(b, &got); err != nil {
		t.Fatalf("decode json: %v", err)
	}
	if got.Sources["papirus"].Provider != "github" {
		t.Fatalf("missing sources lock entry: %#v", got.Sources)
	}
	if got.Modules["alfa"].Version != "v2" {
		t.Fatalf("missing version in content: %#v", got.Modules["alfa"])
	}
}

func TestBuildSourceLockEntries(t *testing.T) {
	t.Parallel()

	mod := &ModFile{
		Sources: map[string]SourceConfig{
			"papirus": {Provider: "github", Repo: "PapirusDevelopmentTeam/papirus-icon-theme"},
		},
	}

	got := BuildSourceLockEntries(mod)
	entry, ok := got["papirus"]
	if !ok {
		t.Fatal("expected papirus source lock")
	}
	if entry.Provider != "github" || entry.Repo != "PapirusDevelopmentTeam/papirus-icon-theme" {
		t.Fatalf("unexpected source lock: %#v", entry)
	}
}
