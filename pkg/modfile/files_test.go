package modfile

import (
	"os"
	"path/filepath"
	"testing"
)

func TestEnsureLockFileCreatesLock(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	sumPath, err := EnsureLockFile(root)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if _, err := os.Stat(sumPath); err != nil {
		t.Fatalf("sum file was not created: %v", err)
	}

	if filepath.Dir(sumPath) != root {
		t.Fatalf("unexpected sum path: %s", sumPath)
	}
}
