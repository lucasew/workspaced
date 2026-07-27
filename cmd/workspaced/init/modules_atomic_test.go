package init

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCopyEmbeddedModulesAtomicWrite(t *testing.T) {
	dir := t.TempDir()
	// Seed a truncated prior file under the example module path if present after copy.
	if err := copyEmbeddedModules(dir); err != nil {
		t.Fatalf("copyEmbeddedModules: %v", err)
	}
	// Ensure no .tmp leftovers
	err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if strings.HasSuffix(path, ".tmp") {
			t.Errorf("temp file left behind: %s", path)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	// Second copy overwrites cleanly
	if err := copyEmbeddedModules(dir); err != nil {
		t.Fatalf("second copyEmbeddedModules: %v", err)
	}
	// At least one regular file was written
	var files int
	if walkErr := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err == nil && !info.IsDir() {
			files++
		}
		return nil
	}); walkErr != nil {
		t.Logf("count walk: %v", walkErr)
	}
	if files == 0 {
		t.Fatal("expected embedded module files")
	}
}
