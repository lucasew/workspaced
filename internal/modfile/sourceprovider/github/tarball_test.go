package github

import (
	"archive/tar"
	"bytes"
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestExtractTarEntryRemovesPartialOnCopyError(t *testing.T) {
	t.Parallel()

	var full bytes.Buffer
	tw := tar.NewWriter(&full)
	content := bytes.Repeat([]byte("x"), 2048)
	if err := tw.WriteHeader(&tar.Header{
		Name: "file.txt",
		Mode: 0o644,
		Size: int64(len(content)),
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := tw.Write(content); err != nil {
		t.Fatal(err)
	}
	if err := tw.Close(); err != nil {
		t.Fatal(err)
	}

	// Full 512-byte header plus a short body so io.Copy hits UnexpectedEOF.
	partial := full.Bytes()[:512+64]
	tr := tar.NewReader(bytes.NewReader(partial))
	hdr, err := tr.Next()
	if err != nil {
		t.Fatalf("Next: %v", err)
	}

	dir := t.TempDir()
	target := filepath.Join(dir, "file.txt")
	err = extractTarEntry(context.Background(), tr, hdr, target)
	if err == nil {
		t.Fatal("expected copy error from truncated tar body")
	}
	if _, statErr := os.Stat(target); !os.IsNotExist(statErr) {
		t.Fatalf("partial file still present after error: stat=%v extract=%v", statErr, err)
	}
}

func TestExtractTarEntryWritesRegularFile(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	tw := tar.NewWriter(&buf)
	content := []byte("hello module")
	if err := tw.WriteHeader(&tar.Header{
		Name: "file.txt",
		Mode: 0o644,
		Size: int64(len(content)),
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := tw.Write(content); err != nil {
		t.Fatal(err)
	}
	if err := tw.Close(); err != nil {
		t.Fatal(err)
	}

	tr := tar.NewReader(&buf)
	hdr, err := tr.Next()
	if err != nil {
		t.Fatalf("Next: %v", err)
	}
	dir := t.TempDir()
	target := filepath.Join(dir, "file.txt")
	if err := extractTarEntry(context.Background(), tr, hdr, target); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(target)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, content) {
		t.Fatalf("got %q, want %q", got, content)
	}
}
