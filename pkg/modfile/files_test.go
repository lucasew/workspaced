package modfile

import (
	"os"
	"path/filepath"
	"testing"
)

func TestEnsureModAndSumFilesCreatesBoth(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	modPath, sumPath, err := EnsureModAndSumFiles(root)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if _, err := os.Stat(modPath); err != nil {
		t.Fatalf("mod file was not created: %v", err)
	}
	if _, err := os.Stat(sumPath); err != nil {
		t.Fatalf("sum file was not created: %v", err)
	}

	if filepath.Dir(modPath) != root {
		t.Fatalf("unexpected mod path: %s", modPath)
	}
	if filepath.Dir(sumPath) != root {
		t.Fatalf("unexpected sum path: %s", sumPath)
	}
}
