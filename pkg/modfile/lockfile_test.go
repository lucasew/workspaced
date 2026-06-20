package modfile

import (
	"path/filepath"
	"testing"
)

func TestWriteSumFile(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	path := filepath.Join(dir, "workspaced.lock.json")

	sum := &SumFile{}
	sum.EnsureSource("papirus", LockedSource{
		Provider: "github",
		Repo:     "PapirusDevelopmentTeam/papirus-icon-theme",
		URL:      "https://codeload.github.com/PapirusDevelopmentTeam/papirus-icon-theme/tar.gz/main",
		Hash:     "abc123",
		Ref:      "main",
	})
	sum.EnsureTool("fd", LockedTool{
		Ref:     "github:sharkdp/fd",
		Version: "v10.4.0",
	})
	err := writeSumFile(t.Context(), path, sum)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	got, err := LoadSumFile(path)
	if err != nil {
		t.Fatalf("load written: %v", err)
	}
	// Lookups now keyed by ref (kind+ref) in the rebuilt locks.
	_, ok := got.FindSource("main")
	if !ok {
		t.Fatalf("missing source lock entry: %#v", got.Dependencies)
	}
	tool, ok := got.FindTool("github:sharkdp/fd")
	if !ok || tool.Version != "v10.4.0" {
		t.Fatalf("missing tool version in content: %#v", got.Dependencies)
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
