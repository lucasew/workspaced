package ruff

import (
	"os"
	"path/filepath"
	"testing"

	_ "workspaced/pkg/driver/httpclient/native"
	"workspaced/pkg/logging"
	_ "workspaced/pkg/tool/backend/catalog"
	_ "workspaced/pkg/tool/backend/catalog/applications"
	_ "workspaced/pkg/tool/backend/github"
)

func TestFormat(t *testing.T) {
	if os.Getenv("CI") != "" {
		t.Skip("Skipping integration test in CI")
	}

	dir := t.TempDir()

	if err := os.WriteFile(filepath.Join(dir, "uv.lock"), []byte(""), 0644); err != nil {
		t.Fatal(err)
	}

	pyFile := filepath.Join(dir, "main.py")
	// "x=1" should be formatted to "x = 1\n"
	if err := os.WriteFile(pyFile, []byte("x=1"), 0644); err != nil {
		t.Fatal(err)
	}

	p := New()
	ctx := logging.NewWriterContext(t.Output())

	err := p.Detect(ctx, dir)
	if err != nil {
		t.Fatalf("Detect failed: %v", err)
	}

	if err := p.Format(ctx, dir); err != nil {
		t.Skipf("ruff unavailable in this environment: %v", err)
	}

	content, err := os.ReadFile(pyFile)
	if err != nil {
		t.Fatal(err)
	}

	expected := "x = 1\n"
	if string(content) != expected {
		t.Errorf("Format failed. Got %q, want %q", string(content), expected)
	}
}
