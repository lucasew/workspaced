package open

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	_ "github.com/lucasew/workspaced/pkg/driver/prelude"
	"github.com/lucasew/workspaced/pkg/logging"
	"errors"
	"io/fs"
)

func TestEnsureMiseWrapperAtomicWrite(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	ctx := logging.NewWriterContext(t.Output())

	wrapperDir := filepath.Join(home, ".local", "bin")
	if err := os.MkdirAll(wrapperDir, 0o755); err != nil {
		t.Fatal(err)
	}
	wrapperPath := filepath.Join(wrapperDir, "mise")
	// Prior truncated/stale wrapper that must be fully replaced.
	if err := os.WriteFile(wrapperPath, []byte("#!/bin/sh\necho prior\n"), 0o755); err != nil {
		t.Fatal(err)
	}

	if err := ensureMiseWrapper(ctx, "unused"); err != nil {
		t.Fatalf("ensureMiseWrapper: %v", err)
	}

	if _, err := os.Stat(wrapperPath + ".tmp"); !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("temp wrapper still present: %v", err)
	}
	content, err := os.ReadFile(wrapperPath)
	if err != nil {
		t.Fatal(err)
	}
	got := string(content)
	if strings.Contains(got, "prior") {
		t.Fatalf("prior content still present:\n%s", got)
	}
	if !strings.Contains(got, "open mise") {
		t.Fatalf("wrapper missing open mise:\n%s", got)
	}
	info, err := os.Stat(wrapperPath)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode()&0o111 == 0 {
		t.Fatalf("wrapper not executable: %o", info.Mode())
	}
}
