package archive

import (
	"bytes"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestPathWithinDest(t *testing.T) {
	t.Parallel()
	dest := filepath.Join(t.TempDir(), "out")
	if err := os.MkdirAll(dest, 0o755); err != nil {
		t.Fatal(err)
	}
	if !PathWithinDest(dest, dest) {
		t.Fatal("dest should be within itself")
	}
	if !PathWithinDest(dest, filepath.Join(dest, "a", "b")) {
		t.Fatal("nested should be within")
	}
	if PathWithinDest(dest, filepath.Join(dest, "..", "escape")) {
		t.Fatal("parent escape should fail")
	}
}

func TestJoinWithinRejectsTraversal(t *testing.T) {
	t.Parallel()
	dest := t.TempDir()
	for _, name := range []string{"../outside", "foo/../../outside", "/abs"} {
		if _, err := JoinWithin(dest, name); err == nil {
			t.Fatalf("expected error for %q", name)
		}
	}
	got, err := JoinWithin(dest, "ok/file.txt")
	if err != nil {
		t.Fatal(err)
	}
	if got != filepath.Join(dest, "ok", "file.txt") {
		t.Fatalf("got %q", got)
	}
}

func TestWriteMemberRemovesPartial(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	path := filepath.Join(dir, "file.txt")
	r := io.MultiReader(
		bytes.NewReader([]byte("partial-")),
		errReader{errors.New("boom")},
	)
	if err := WriteMember(path, 0o644, r); err == nil {
		t.Fatal("expected error")
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("partial still present: %v", err)
	}
}

func TestWriteMemberSuccess(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	path := filepath.Join(dir, "bin", "tool")
	if err := WriteMember(path, 0o755, bytes.NewReader([]byte("#!/bin/sh\n"))); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm()&0o111 == 0 {
		t.Fatalf("expected executable, mode=%o", info.Mode().Perm())
	}
}

func TestSymlinkTargetWithin(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	link := filepath.Join(dir, "link")
	if !SymlinkTargetWithin(dir, link, "sibling.txt") {
		t.Fatal("in-dest relative should be ok")
	}
	if SymlinkTargetWithin(dir, link, "../../outside") {
		t.Fatal("escaping relative should fail")
	}
	if SymlinkTargetWithin(dir, link, "") {
		t.Fatal("empty should fail")
	}
}

func TestResolveWithin(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	nested := filepath.Join(dir, "a")
	if err := os.MkdirAll(nested, 0o755); err != nil {
		t.Fatal(err)
	}
	target := filepath.Join(nested, "f.txt")
	got, err := ResolveWithin(dir, target)
	if err != nil {
		t.Fatal(err)
	}
	if got != target && filepath.Clean(got) != filepath.Clean(target) {
		// EvalSymlinks may rewrite; still under dest
		if !PathWithinDest(dir, got) {
			t.Fatalf("resolved outside: %q", got)
		}
	}
	// Symlink parent that escapes: dir/sub -> /tmp or parent
	escape := filepath.Join(dir, "escape-link")
	if err := os.Symlink("..", escape); err != nil {
		t.Fatal(err)
	}
	bad := filepath.Join(escape, "x")
	if _, err := ResolveWithin(dir, bad); err == nil {
		t.Fatal("expected escape via symlink parent")
	} else if !strings.Contains(err.Error(), "illegal") {
		// may be EvalSymlinks or illegal
		t.Logf("got err %v (ok if non-nil)", err)
	}
}

type errReader struct{ err error }

func (e errReader) Read([]byte) (int, error) { return 0, e.err }
