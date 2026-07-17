package deployer

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestFileStateStoreRelativeToRoot(t *testing.T) {
	dir := t.TempDir()
	root := filepath.Join(dir, "root")
	if err := os.MkdirAll(root, 0o755); err != nil {
		t.Fatal(err)
	}
	statePath := filepath.Join(dir, "state.json")

	store, err := NewFileStateStore(statePath, root)
	if err != nil {
		t.Fatalf("NewFileStateStore: %v", err)
	}

	absA := filepath.Join(root, "a", "file.txt")
	absB := filepath.Join(root, "b.txt")
	in := &State{Files: map[string]ManagedInfo{
		absA: {SourceInfo: "mod:a"},
		absB: {SourceInfo: "mod:b"},
	}}
	if err := store.Save(in); err != nil {
		t.Fatalf("Save: %v", err)
	}

	raw, err := os.ReadFile(statePath)
	if err != nil {
		t.Fatal(err)
	}
	var disk State
	if err := json.Unmarshal(raw, &disk); err != nil {
		t.Fatal(err)
	}
	if _, ok := disk.Files[filepath.Join("a", "file.txt")]; !ok {
		t.Fatalf("expected relative key a/file.txt in disk state, got %#v", disk.Files)
	}
	if _, ok := disk.Files["b.txt"]; !ok {
		t.Fatalf("expected relative key b.txt in disk state, got %#v", disk.Files)
	}
	for k := range disk.Files {
		if filepath.IsAbs(k) {
			t.Fatalf("disk key should be relative, got %q", k)
		}
	}

	loaded, err := store.Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if info, ok := loaded.Files[absA]; !ok || info.SourceInfo != "mod:a" {
		t.Fatalf("load absA: got %#v", loaded.Files)
	}
	if info, ok := loaded.Files[absB]; !ok || info.SourceInfo != "mod:b" {
		t.Fatalf("load absB: got %#v", loaded.Files)
	}
}

func TestFileStateStoreSaveIsAtomic(t *testing.T) {
	dir := t.TempDir()
	root := filepath.Join(dir, "root")
	if err := os.MkdirAll(root, 0o755); err != nil {
		t.Fatal(err)
	}
	statePath := filepath.Join(dir, "state.json")
	// Pre-existing state that must remain readable if replace is atomic.
	old := &State{Files: map[string]ManagedInfo{
		filepath.Join(root, "old.txt"): {SourceInfo: "old"},
	}}
	store, err := NewFileStateStore(statePath, root)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.Save(old); err != nil {
		t.Fatal(err)
	}

	next := &State{Files: map[string]ManagedInfo{
		filepath.Join(root, "new.txt"): {SourceInfo: "new"},
	}}
	if err := store.Save(next); err != nil {
		t.Fatalf("Save: %v", err)
	}

	if _, err := os.Stat(statePath + ".tmp"); !os.IsNotExist(err) {
		t.Fatalf("temp file should be gone after successful Save, err=%v", err)
	}

	raw, err := os.ReadFile(statePath)
	if err != nil {
		t.Fatal(err)
	}
	var disk State
	if err := json.Unmarshal(raw, &disk); err != nil {
		t.Fatalf("final state must be valid JSON: %v\n%s", err, raw)
	}
	if _, ok := disk.Files["new.txt"]; !ok {
		t.Fatalf("expected new.txt after save, got %#v", disk.Files)
	}
	if _, ok := disk.Files["old.txt"]; ok {
		t.Fatalf("old.txt should have been replaced, got %#v", disk.Files)
	}
}

func TestFileStateStoreMigratesAbsoluteKeys(t *testing.T) {
	dir := t.TempDir()
	root := filepath.Join(dir, "root")
	if err := os.MkdirAll(root, 0o755); err != nil {
		t.Fatal(err)
	}
	statePath := filepath.Join(dir, "state.json")
	abs := filepath.Join(root, "legacy.txt")

	// Legacy on-disk format: absolute keys.
	legacy := &State{Files: map[string]ManagedInfo{
		abs: {SourceInfo: "old"},
	}}
	data, err := json.MarshalIndent(legacy, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(statePath, data, 0o644); err != nil {
		t.Fatal(err)
	}

	store, err := NewFileStateStore(statePath, root)
	if err != nil {
		t.Fatal(err)
	}
	loaded, err := store.Load()
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := loaded.Files[abs]; !ok {
		t.Fatalf("expected absolute key after load, got %#v", loaded.Files)
	}

	// Re-save should rewrite as relative.
	if err := store.Save(loaded); err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(statePath)
	if err != nil {
		t.Fatal(err)
	}
	var disk State
	if err := json.Unmarshal(raw, &disk); err != nil {
		t.Fatal(err)
	}
	if _, ok := disk.Files["legacy.txt"]; !ok {
		t.Fatalf("expected relativized key after save, got %#v", disk.Files)
	}
}

func TestRelToRootAndAbsFromRoot(t *testing.T) {
	root := "/home/user"
	if got := RelToRoot("/home/user/.config/foo", root); got != ".config/foo" {
		t.Fatalf("RelToRoot: got %q", got)
	}
	if got := AbsFromRoot(".config/foo", root); got != "/home/user/.config/foo" {
		t.Fatalf("AbsFromRoot: got %q", got)
	}
	// Outside root stays absolute.
	if got := RelToRoot("/other/x", root); got != "/other/x" {
		t.Fatalf("RelToRoot outside: got %q", got)
	}
}

func TestPrettyPathUsesRelToRoot(t *testing.T) {
	home, err := os.UserHomeDir()
	if err != nil {
		t.Fatal(err)
	}
	got := PrettyPath(filepath.Join(home, ".config", "x"))
	if got != "~/.config/x" {
		t.Fatalf("PrettyPath: got %q", got)
	}
}
