package init

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/lucasew/workspaced/pkg/logging"
)

func TestGenerateConfigAtomicWrite(t *testing.T) {
	ctx := logging.NewWriterContext(t.Output())
	dir := t.TempDir()
	configPath := filepath.Join(dir, "workspaced.cue")

	// Seed an existing config so --force-style overwrite cannot wipe it on failure
	// of a later stage. generateConfig always targets configPath via temp+rename.
	const prior = "// prior config must survive a failed write path\n"
	if err := os.WriteFile(configPath, []byte(prior), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := generateConfig(ctx, configPath); err != nil {
		t.Fatalf("generateConfig: %v", err)
	}

	got, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) == prior {
		t.Fatal("config was not replaced with template output")
	}
	if !strings.Contains(string(got), "workspaced:") || !strings.Contains(string(got), "modules:") {
		t.Fatalf("unexpected template output: %q", got)
	}
	// Temp must not linger after success.
	if _, err := os.Stat(configPath + ".tmp"); !os.IsNotExist(err) {
		t.Fatalf("temp file still present, err=%v", err)
	}
}

func TestGenerateConfigRemovesTempOnSuccess(t *testing.T) {
	ctx := logging.NewWriterContext(t.Output())
	dir := t.TempDir()
	configPath := filepath.Join(dir, "workspaced.cue")

	if err := generateConfig(ctx, configPath); err != nil {
		t.Fatalf("generateConfig: %v", err)
	}
	info, err := os.Stat(configPath)
	if err != nil {
		t.Fatal(err)
	}
	if info.Size() == 0 {
		t.Fatal("config is empty")
	}
	if _, err := os.Stat(configPath + ".tmp"); !os.IsNotExist(err) {
		t.Fatalf("temp file still present after success, err=%v", err)
	}
}
