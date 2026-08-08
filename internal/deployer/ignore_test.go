package deployer

import (
	"os"
	"path/filepath"
	"testing"
)

func writeGitWorkTree(t *testing.T, gitignore string) string {
	t.Helper()
	root := t.TempDir()
	if err := os.Mkdir(filepath.Join(root, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	if gitignore != "" {
		if err := os.WriteFile(filepath.Join(root, ".gitignore"), []byte(gitignore), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return root
}

func TestGitignoreUntrackedWalksUpToGitRoot(t *testing.T) {
	t.Parallel()
	gitRoot := writeGitWorkTree(t, ".grok/\n")
	nested := filepath.Join(gitRoot, "apps", "svc")
	if err := os.MkdirAll(nested, 0o755); err != nil {
		t.Fatal(err)
	}
	ignore := GitignoreUntracked(nested)
	if ignore == nil {
		t.Fatal("expected ignore fn from parent git root")
	}
	if !ignore(filepath.Join(nested, ".grok", "skill.md")) {
		t.Fatal("nested .grok path should match parent .gitignore")
	}
	if ignore(filepath.Join(nested, "main.go")) {
		t.Fatal("nested tracked path should not be ignored")
	}
}

func TestGitignoreUntrackedNilWithoutGit(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, ".gitignore"), []byte(".grok/\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if got := GitignoreUntracked(root); got != nil {
		t.Fatal("expected nil ignore without .git")
	}
}

func TestGitignoreUntrackedMatchesRepoIgnore(t *testing.T) {
	t.Parallel()
	root := writeGitWorkTree(t, ".grok/\n*.local\n!keep.local\n")
	ignore := GitignoreUntracked(root)
	if ignore == nil {
		t.Fatal("expected ignore fn")
	}

	tests := []struct {
		rel  string
		want bool
	}{
		{".grok/skills/foo.md", true},
		{"keep.local", false},
		{"foo.local", true},
		{"README.md", false},
	}
	for _, tt := range tests {
		got := ignore(filepath.Join(root, filepath.FromSlash(tt.rel)))
		if got != tt.want {
			t.Errorf("ignore(%q)=%v want %v", tt.rel, got, tt.want)
		}
	}
	if ignore(filepath.Join(t.TempDir(), "outside")) {
		t.Error("path outside root should not be ignored")
	}
}

func TestGitignoreUntrackedNestedAndExclude(t *testing.T) {
	t.Parallel()
	root := writeGitWorkTree(t, "")
	if err := os.MkdirAll(filepath.Join(root, ".git", "info"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, ".git", "info", "exclude"), []byte("scratch/\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	nested := filepath.Join(root, "pkg")
	if err := os.MkdirAll(nested, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(nested, ".gitignore"), []byte("gen/\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	ignore := GitignoreUntracked(root)
	if ignore == nil {
		t.Fatal("expected ignore fn")
	}
	if !ignore(filepath.Join(root, "scratch", "a.txt")) {
		t.Error("info/exclude scratch/ should ignore")
	}
	if !ignore(filepath.Join(root, "pkg", "gen", "out.go")) {
		t.Error("nested pkg/.gitignore gen/ should ignore")
	}
	if ignore(filepath.Join(root, "pkg", "main.go")) {
		t.Error("pkg/main.go should not be ignored")
	}
}

func TestDropIgnored(t *testing.T) {
	t.Parallel()
	root := writeGitWorkTree(t, ".grok/\n")
	ignore := GitignoreUntracked(root)
	tracked := filepath.Join(root, "README.md")
	untracked := filepath.Join(root, ".grok", "a.md")
	state := &State{Files: map[string]ManagedInfo{
		tracked:   {SourceInfo: "a"},
		untracked: {SourceInfo: "b"},
	}}
	n := DropIgnored(state, ignore)
	if n != 1 {
		t.Fatalf("dropped %d want 1", n)
	}
	if _, ok := state.Files[tracked]; !ok {
		t.Fatal("tracked key dropped")
	}
	if _, ok := state.Files[untracked]; ok {
		t.Fatal("ignored key still in state")
	}
}
