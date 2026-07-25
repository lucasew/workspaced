package wm

import (
	"os"
	"path/filepath"
	"testing"
)

func TestWriteLastWSFileAtomic(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "last_ws")
	if err := os.WriteFile(path, []byte("10"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := writeLastWSFile(path, "11"); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(path + ".tmp"); !os.IsNotExist(err) {
		t.Fatalf("tmp still present: %v", err)
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(raw) != "11" {
		t.Fatalf("content=%q", raw)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Fatalf("mode=%o", info.Mode().Perm())
	}
}
