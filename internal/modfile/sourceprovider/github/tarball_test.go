package github

import (
	"archive/tar"
	"bytes"
	"github.com/lucasew/workspaced/internal/archive"
	"os"
	"path/filepath"
	"strings"
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
	err = extractTarEntry(t.Context(), tr, hdr, dir, target)
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
	if err := extractTarEntry(t.Context(), tr, hdr, dir, target); err != nil {
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

func TestMapTarEntryTargetRejectsPathTraversal(t *testing.T) {
	t.Parallel()

	dest := t.TempDir()
	cases := []string{
		"repo-sha/../../outside.txt",
		"repo-sha/foo/../../../outside.txt",
		"./repo-sha/../escape",
	}
	for _, name := range cases {
		_, skip, err := mapTarEntryTarget(name, dest)
		if err == nil {
			t.Fatalf("name %q: expected illegal path error, skip=%v", name, skip)
		}
		if !strings.Contains(err.Error(), "illegal file path") {
			t.Fatalf("name %q: got %v, want illegal file path", name, err)
		}
	}
}

func TestMapTarEntryTargetAllowsSafeNested(t *testing.T) {
	t.Parallel()

	dest := t.TempDir()
	target, skip, err := mapTarEntryTarget("repo-sha/subdir/file.txt", dest)
	if err != nil {
		t.Fatal(err)
	}
	if skip {
		t.Fatal("expected map, not skip")
	}
	want := filepath.Join(dest, "subdir", "file.txt")
	if target != want {
		t.Fatalf("target=%q want=%q", target, want)
	}
	if !archive.PathWithinDest(dest, target) {
		t.Fatalf("mapped target not within dest: %q", target)
	}
}

func TestMapTarEntryTargetSkipsPrefixOnly(t *testing.T) {
	t.Parallel()

	_, skip, err := mapTarEntryTarget("repo-sha", t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if !skip {
		t.Fatal("expected skip for prefix-only entry")
	}
}

func TestExtractTarEntryRejectsEscapingSymlink(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	target := filepath.Join(dir, "link")
	hdr := &tar.Header{
		Typeflag: tar.TypeSymlink,
		Name:     "repo-sha/link",
		Linkname: "../../outside",
	}
	err := extractTarEntry(t.Context(), nil, hdr, dir, target)
	if err == nil {
		t.Fatal("expected illegal symlink target error")
	}
	if !strings.Contains(err.Error(), "illegal symlink") {
		t.Fatalf("got %v, want illegal symlink", err)
	}
	if _, statErr := os.Lstat(target); !os.IsNotExist(statErr) {
		t.Fatalf("symlink should not exist: %v", statErr)
	}
}

func TestExtractTarEntryAllowsInDestSymlink(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	target := filepath.Join(dir, "link")
	hdr := &tar.Header{
		Typeflag: tar.TypeSymlink,
		Name:     "repo-sha/link",
		Linkname: "sibling.txt",
	}
	if err := extractTarEntry(t.Context(), nil, hdr, dir, target); err != nil {
		t.Fatal(err)
	}
	got, err := os.Readlink(target)
	if err != nil {
		t.Fatal(err)
	}
	if got != "sibling.txt" {
		t.Fatalf("linkname=%q", got)
	}
}
