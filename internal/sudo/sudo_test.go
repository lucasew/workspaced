package sudo

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/lucasew/workspaced/internal/types"
	_ "github.com/lucasew/workspaced/pkg/driver/prelude"
	"github.com/lucasew/workspaced/pkg/logging"
	"errors"
	"io/fs"
)

func TestQueuePathRejectsTraversal(t *testing.T) {
	dir := t.TempDir()
	for _, slug := range []string{
		"../escape",
		"..",
		"foo/bar",
		"foo\\bar",
		"",
		".",
		"a/../../etc/passwd",
	} {
		if _, err := queuePath(dir, slug); err == nil {
			t.Fatalf("queuePath(%q) accepted traversal/invalid slug", slug)
		}
	}
}

func TestQueuePathAcceptsSimpleSlug(t *testing.T) {
	dir := t.TempDir()
	p, err := queuePath(dir, "abc123")
	if err != nil {
		t.Fatalf("queuePath: %v", err)
	}
	want := filepath.Join(dir, "abc123.json")
	if p != want {
		t.Fatalf("path=%q want %q", p, want)
	}
}

func TestEnqueueJailsSlugAndMode(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	ctx := logging.NewWriterContext(t.Output())

	// Malicious slug must not write outside queue dir.
	err := Enqueue(ctx, &types.SudoCommand{
		Slug:    "../escape",
		Command: "true",
		Env:     []string{"SECRET=s3cr3t"},
	})
	if err == nil {
		t.Fatal("expected error for path-escaping slug")
	}
	// No escape file next to queue parent
	if _, err := os.Stat(filepath.Join(home, ".cache/workspaced/escape.json")); err == nil {
		t.Fatal("escaped write created file outside queue")
	}

	err = Enqueue(ctx, &types.SudoCommand{
		Slug:    "safe1",
		Command: "true",
		Env:     []string{"SECRET=s3cr3t"},
	})
	if err != nil {
		t.Fatalf("Enqueue: %v", err)
	}
	path := filepath.Join(home, ".cache/workspaced/sudo_queue/safe1.json")
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat queue file: %v", err)
	}
	if perm := info.Mode().Perm(); perm != 0o600 {
		t.Fatalf("queue file mode=%o want 0600", perm)
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var got types.SudoCommand
	if err := json.Unmarshal(raw, &got); err != nil {
		t.Fatal(err)
	}
	if got.Command != "true" || !strings.Contains(strings.Join(got.Env, "\n"), "SECRET=s3cr3t") {
		t.Fatalf("unexpected payload: %+v", got)
	}

	// Get / Remove use same jail
	if _, err := Get("../escape"); err == nil {
		t.Fatal("Get accepted escaping slug")
	}
	if err := Remove("../escape"); err == nil {
		t.Fatal("Remove accepted escaping slug")
	}
	if err := Remove("safe1"); err != nil {
		t.Fatalf("Remove: %v", err)
	}
	if _, err := os.Stat(path); !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("file still present after Remove: %v", err)
	}
}

func TestGetQueueDirMode(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	dir, err := getQueueDir()
	if err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(dir)
	if err != nil {
		t.Fatal(err)
	}
	if perm := info.Mode().Perm(); perm != 0o700 {
		t.Fatalf("queue dir mode=%o want 0700", perm)
	}
}
