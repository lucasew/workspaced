package atomicfile

import (
	"bytes"
	"errors"
	"image"
	"image/color"
	"image/png"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestWriteBytesSuccess(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	path := filepath.Join(dir, "out.txt")
	if err := WriteBytes(path, []byte("hello"), 0o644); err != nil {
		t.Fatal(err)
	}
	assertNoTmpLeft(t, dir, filepath.Base(path))
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "hello" {
		t.Fatalf("got %q", got)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o644 {
		t.Fatalf("mode=%o", info.Mode().Perm())
	}
}

func TestWriteString(t *testing.T) {
	t.Parallel()
	path := filepath.Join(t.TempDir(), "s.txt")
	if err := WriteString(path, "hi", 0o600); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "hi" {
		t.Fatalf("got %q", got)
	}
}

func TestWriteFailureKeepsExisting(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	path := filepath.Join(dir, "cache.bin")
	if err := os.WriteFile(path, []byte("good"), 0o644); err != nil {
		t.Fatal(err)
	}
	r := io.MultiReader(
		bytes.NewReader([]byte("partial-")),
		errReader{errors.New("boom")},
	)
	if err := Write(path, r, 0o644); err == nil {
		t.Fatal("expected error")
	}
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "good" {
		t.Fatalf("prior content lost: %q", got)
	}
	assertNoTmpLeft(t, dir, filepath.Base(path))
}

func TestWriteFailureLeavesNoFinal(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	path := filepath.Join(dir, "new.bin")
	if err := Write(path, errReader{errors.New("boom")}, 0o644); err == nil {
		t.Fatal("expected error")
	}
	if _, err := os.Stat(path); !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("final path should not exist: %v", err)
	}
	assertNoTmpLeft(t, dir, filepath.Base(path))
}

func TestWritePNG(t *testing.T) {
	t.Parallel()
	path := filepath.Join(t.TempDir(), "shot.png")
	if err := os.WriteFile(path, []byte("truncated"), 0o644); err != nil {
		t.Fatal(err)
	}
	img := image.NewRGBA(image.Rect(0, 0, 2, 2))
	img.Set(0, 0, color.RGBA{R: 255, A: 255})
	if err := WritePNG(path, img); err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(raw) < 8 || string(raw[:8]) != "\x89PNG\r\n\x1a\n" {
		t.Fatalf("not a PNG header: %q", raw[:min(8, len(raw))])
	}
}

func TestCreateCommitEncode(t *testing.T) {
	t.Parallel()
	path := filepath.Join(t.TempDir(), "shot.png")
	if err := os.WriteFile(path, []byte("truncated"), 0o644); err != nil {
		t.Fatal(err)
	}
	img := image.NewRGBA(image.Rect(0, 0, 2, 2))
	img.Set(0, 0, color.RGBA{R: 255, A: 255})

	f, err := Create(path, 0o644)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Abort()
	if f.Name() == SiblingTemp(path) {
		t.Fatal("Create should use a unique temp name, not SiblingTemp")
	}
	if err := png.Encode(f, img); err != nil {
		t.Fatal(err)
	}
	if err := f.Commit(); err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(raw) < 8 || string(raw[:8]) != "\x89PNG\r\n\x1a\n" {
		t.Fatalf("not a PNG: %q", raw[:min(16, len(raw))])
	}
}

func TestCreateSibling(t *testing.T) {
	t.Parallel()
	path := filepath.Join(t.TempDir(), "out.txt")
	f, err := CreateSibling(path, 0o600)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Abort()
	if f.Name() != SiblingTemp(path) {
		t.Fatalf("temp=%q want %q", f.Name(), SiblingTemp(path))
	}
	if _, err := io.WriteString(f, "x"); err != nil {
		t.Fatal(err)
	}
	if err := f.Commit(); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(SiblingTemp(path)); !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("sibling tmp left: %v", err)
	}
}

func TestCreateMode(t *testing.T) {
	t.Parallel()
	path := filepath.Join(t.TempDir(), "secret.json")
	f, err := Create(path, 0o600)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Abort()
	if _, err := io.WriteString(f, `{"a":1}`); err != nil {
		t.Fatal(err)
	}
	if err := f.Commit(); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Fatalf("mode=%o", info.Mode().Perm())
	}
}

func TestInstallChmod(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	tmp := filepath.Join(dir, "x.tmp")
	dest := filepath.Join(dir, "x")
	if err := os.WriteFile(tmp, []byte("bin"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := Install(tmp, dest, 0o755); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(dest)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o755 {
		t.Fatalf("mode=%o", info.Mode().Perm())
	}
}

func TestReplaceDir(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	dest := filepath.Join(root, "v1")
	if err := os.MkdirAll(dest, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dest, "old"), []byte("old"), 0o644); err != nil {
		t.Fatal(err)
	}
	tmp := filepath.Join(root, "v1.tmp")
	if err := os.MkdirAll(tmp, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(tmp, "new"), []byte("new"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := ReplaceDir(dest, tmp); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(dest, "new")); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(dest, "old")); !errors.Is(err, fs.ErrNotExist) {
		t.Fatal("old content still present")
	}
}

func assertNoTmpLeft(t *testing.T, dir, base string) {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	prefix := base + ".tmp"
	for _, e := range entries {
		name := e.Name()
		if name == prefix || strings.HasPrefix(name, prefix+"-") {
			t.Fatalf("tmp left behind: %s", name)
		}
	}
}

type errReader struct{ err error }

func (e errReader) Read([]byte) (int, error) { return 0, e.err }
