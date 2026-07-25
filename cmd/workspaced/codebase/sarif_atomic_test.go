package codebase

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/owenrumney/go-sarif/v2/sarif"
)

func TestWriteSarifAtomic(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "lint.sarif")
	if err := os.WriteFile(path, []byte("{broken"), 0o644); err != nil {
		t.Fatal(err)
	}
	report := &sarif.Report{Version: "2.1.0"}
	if err := writeSarifAtomic(path, report); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(path + ".tmp"); !os.IsNotExist(err) {
		t.Fatalf("tmp left: %v", err)
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(raw) == 0 || raw[0] != '{' {
		t.Fatalf("bad sarif: %q", raw[:min(20, len(raw))])
	}
}
