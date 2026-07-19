package resolution

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/lucasew/workspaced/pkg/logging"
)

func TestReadToolVersion_found(t *testing.T) {
	ctx := logging.NewWriterContext(t.Output())
	dir := t.TempDir()
	path := filepath.Join(dir, ".tool-versions")
	if err := os.WriteFile(path, []byte("# comment\n\ngo 1.22.0\ndeno 1.40.0\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	got, err := readToolVersion(ctx, path, "deno")
	if err != nil {
		t.Fatalf("readToolVersion: %v", err)
	}
	if got != "1.40.0" {
		t.Fatalf("got %q, want 1.40.0", got)
	}
}

func TestReadToolVersion_missing(t *testing.T) {
	ctx := logging.NewWriterContext(t.Output())
	dir := t.TempDir()
	path := filepath.Join(dir, ".tool-versions")
	if err := os.WriteFile(path, []byte("go 1.22.0\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	got, err := readToolVersion(ctx, path, "deno")
	if err != nil {
		t.Fatalf("readToolVersion: %v", err)
	}
	if got != "" {
		t.Fatalf("got %q, want empty", got)
	}
}

func TestReadToolVersion_tokenTooLong(t *testing.T) {
	ctx := logging.NewWriterContext(t.Output())
	dir := t.TempDir()
	path := filepath.Join(dir, ".tool-versions")
	// bufio.Scanner default max token size is 64KiB; one line longer than that fails.
	longLine := strings.Repeat("x", 70*1024) + "\n"
	if err := os.WriteFile(path, []byte(longLine), 0o644); err != nil {
		t.Fatal(err)
	}

	got, err := readToolVersion(ctx, path, "deno")
	if err == nil {
		t.Fatal("expected scan error for oversized line")
	}
	if got != "" {
		t.Fatalf("got %q, want empty", got)
	}
	if !strings.Contains(err.Error(), "scan") {
		t.Fatalf("error = %v, want scan context", err)
	}
}

func TestReadToolVersion_openMissing(t *testing.T) {
	ctx := logging.NewWriterContext(t.Output())
	_, err := readToolVersion(ctx, filepath.Join(t.TempDir(), "nope"), "deno")
	if err == nil {
		t.Fatal("expected open error")
	}
}
