package ruff

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	_ "workspaced/pkg/driver/httpclient/native"
	_ "workspaced/pkg/tool/provider/github"
)

func TestFormat(t *testing.T) {
	if os.Getenv("CI") != "" {
		t.Skip("Skipping integration test in CI")
	}

	// Create a temporary directory for the test project
	dir := t.TempDir()

	// Create uv.lock to trigger detection
	if err := os.WriteFile(filepath.Join(dir, "uv.lock"), []byte(""), 0644); err != nil {
		t.Fatal(err)
	}

	// Create a poorly formatted Python file
	pyFile := filepath.Join(dir, "main.py")
	// "x=1" should be formatted to "x = 1\n"
	if err := os.WriteFile(pyFile, []byte("x=1"), 0644); err != nil {
		t.Fatal(err)
	}

	p := New()
	ctx := context.Background()

	// Verify detection
	err := p.Detect(ctx, dir)
	if err != nil {
		t.Fatalf("Detect failed: %v", err)
	}

	// Run format
	if err := p.Format(ctx, dir); err != nil {
		t.Fatalf("Format failed: %v", err)
	}

	// Verify the file was formatted
	content, err := os.ReadFile(pyFile)
	if err != nil {
		t.Fatal(err)
	}

	expected := "x = 1\n"
	if string(content) != expected {
		t.Errorf("Format failed. Got %q, want %q", string(content), expected)
	}
}
