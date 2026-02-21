package module

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"workspaced/pkg/config"
)

func TestBuildLockEntriesSkipsCoreAndLocal(t *testing.T) {
	t.Parallel()

	cfg := &config.GlobalConfig{
		Modules: map[string]any{
			"icons": map[string]any{"enable": true},
			"foo":   map[string]any{"enable": true, "from": "github:owner/repo/path@v1.0.0"},
			"bar":   map[string]any{"enable": true, "from": "local:bar"},
		},
	}
	modFile := &ModFile{
		Modules: map[string]string{
			"icons": "core:base16-icons-linux",
		},
	}

	got, err := BuildLockEntries(cfg, modFile, "/tmp/modules")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if _, ok := got["icons"]; ok {
		t.Fatalf("icons should not be lock entry for core provider")
	}
	if _, ok := got["bar"]; ok {
		t.Fatalf("bar should not be lock entry for local provider")
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
	path := filepath.Join(dir, "workspaced.sum.toml")

	err := WriteSumFile(path, &SumFile{
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
	content := string(b)

	alfaPos := strings.Index(content, "[modules.alfa]")
	zetaPos := strings.Index(content, "[modules.zeta]")
	if alfaPos < 0 || zetaPos < 0 || alfaPos >= zetaPos {
		t.Fatalf("entries are not sorted: %s", content)
	}
	if !strings.Contains(content, `version = "v2"`) {
		t.Fatalf("missing version in content: %s", content)
	}
}
