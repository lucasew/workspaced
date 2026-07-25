package media

import (
	"bytes"
	"errors"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/lucasew/workspaced/pkg/logging"
)

func TestWriteReaderToFileAtomic_Success(t *testing.T) {
	ctx := logging.NewWriterContext(t.Output())
	dir := t.TempDir()
	path := filepath.Join(dir, "art-cache")

	const body = "new-art-bytes"
	if err := writeReaderToFileAtomic(ctx, bytes.NewReader([]byte(body)), path); err != nil {
		t.Fatalf("writeReaderToFileAtomic: %v", err)
	}

	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != body {
		t.Fatalf("final content = %q, want %q", got, body)
	}
	if _, err := os.Stat(path + ".tmp"); !os.IsNotExist(err) {
		t.Fatalf("temp file still present, err=%v", err)
	}
}

func TestWriteReaderToFileAtomic_FailureKeepsExisting(t *testing.T) {
	ctx := logging.NewWriterContext(t.Output())
	dir := t.TempDir()
	path := filepath.Join(dir, "art-cache")

	const prior = "prior-good-art"
	if err := os.WriteFile(path, []byte(prior), 0o644); err != nil {
		t.Fatal(err)
	}

	r := io.MultiReader(
		bytes.NewReader([]byte("partial-")),
		errReader{err: errors.New("simulated download failure")},
	)
	if err := writeReaderToFileAtomic(ctx, r, path); err == nil {
		t.Fatal("expected error from failing reader")
	}

	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != prior {
		t.Fatalf("existing cache was mutated: got %q, want %q", got, prior)
	}
	if _, err := os.Stat(path + ".tmp"); !os.IsNotExist(err) {
		t.Fatalf("temp file still present after failure, err=%v", err)
	}
}

func TestWriteReaderToFileAtomic_FailureLeavesNoFinal(t *testing.T) {
	ctx := logging.NewWriterContext(t.Output())
	dir := t.TempDir()
	path := filepath.Join(dir, "art-cache")

	r := io.MultiReader(
		bytes.NewReader([]byte("partial-")),
		errReader{err: errors.New("simulated download failure")},
	)
	if err := writeReaderToFileAtomic(ctx, r, path); err == nil {
		t.Fatal("expected error from failing reader")
	}

	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("final path should not exist after failed first write, err=%v", err)
	}
	if _, err := os.Stat(path + ".tmp"); !os.IsNotExist(err) {
		t.Fatalf("temp file still present after failure, err=%v", err)
	}
}

type errReader struct {
	err error
}

func (e errReader) Read([]byte) (int, error) {
	return 0, e.err
}
