package modfile

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestWriteSumFile(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	path := filepath.Join(dir, "workspaced.lock.json")

	err := WriteSumFile(path, &SumFile{
		Dependencies: []RenovateDependency{
			{
				Kind:     "source",
				Name:     "papirus",
				Provider: "github",
				Repo:     "PapirusDevelopmentTeam/papirus-icon-theme",
				URL:      "https://codeload.github.com/PapirusDevelopmentTeam/papirus-icon-theme/tar.gz/main",
				Hash:     "abc123",
			},
			{
				Kind:    "tool",
				Name:    "fd",
				Ref:     "github:sharkdp/fd",
				Version: "v10.4.0",
			},
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
	source, ok := got.FindSource("papirus")
	if !ok || source.Provider != "github" {
		t.Fatalf("missing source lock entry: %#v", got.Dependencies)
	}
	tool, ok := got.FindTool("fd")
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
