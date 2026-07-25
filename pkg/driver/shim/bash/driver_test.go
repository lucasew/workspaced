package bash_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	_ "github.com/lucasew/workspaced/pkg/driver/prelude"
	"github.com/lucasew/workspaced/pkg/driver/shim"
	"github.com/lucasew/workspaced/pkg/logging"
)

func TestGenerateWritesViaTempRename(t *testing.T) {
	ctx := logging.NewWriterContext(t.Output())
	dir := t.TempDir()
	path := filepath.Join(dir, "tool")

	prior := "#!/bin/sh\necho prior\n"
	if err := os.WriteFile(path, []byte(prior), 0o755); err != nil {
		t.Fatalf("seed prior shim: %v", err)
	}

	target := filepath.Join(dir, "real-bin")
	if err := shim.Generate(ctx, path, []string{target}); err != nil {
		t.Fatalf("Generate: %v", err)
	}

	if _, err := os.Stat(path + ".tmp"); !os.IsNotExist(err) {
		t.Fatalf("temp path still present after success: %v", err)
	}

	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read shim: %v", err)
	}
	got := string(content)
	if strings.Contains(got, "prior") {
		t.Fatalf("prior content still present:\n%s", got)
	}
	if !strings.Contains(got, target) {
		t.Fatalf("shim missing target %q:\n%s", target, got)
	}

	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat shim: %v", err)
	}
	if info.Mode()&0o111 == 0 {
		t.Fatalf("shim not executable: %o", info.Mode())
	}
}

func TestGeneratePreservesExistingOnTempWriteFailure(t *testing.T) {
	ctx := logging.NewWriterContext(t.Output())
	dir := t.TempDir()
	path := filepath.Join(dir, "tool")

	prior := "#!/bin/sh\necho keep-me\n"
	if err := os.WriteFile(path, []byte(prior), 0o755); err != nil {
		t.Fatalf("seed prior shim: %v", err)
	}

	// Make the directory non-writable so creating path+".tmp" fails while the
	// existing final path remains readable. Restore perms in cleanup so TempDir
	// removal succeeds.
	if err := os.Chmod(dir, 0o555); err != nil {
		t.Fatalf("chmod dir read-only: %v", err)
	}
	t.Cleanup(func() {
		_ = os.Chmod(dir, 0o755)
	})

	err := shim.Generate(ctx, path, []string{"/bin/true"})
	if err == nil {
		t.Fatal("expected Generate to fail when temp cannot be written")
	}

	// Restore write so we can read the preserved final path.
	if err := os.Chmod(dir, 0o755); err != nil {
		t.Fatalf("restore dir perms: %v", err)
	}

	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read preserved shim: %v", err)
	}
	if string(content) != prior {
		t.Fatalf("existing shim was modified on failed write:\ngot %q\nwant %q", content, prior)
	}
	if _, err := os.Stat(path + ".tmp"); !os.IsNotExist(err) {
		t.Fatalf("temp path left behind after failure: %v", err)
	}
}
