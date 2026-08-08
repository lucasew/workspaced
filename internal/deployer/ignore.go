package deployer

import (
	"io/fs"
	"os"
	"path/filepath"

	"github.com/git-pkgs/gitignore"
)

// GitignoreUntracked returns a predicate that is true when absTarget is
// ignored by the git worktree at root (.gitignore files and .git/info/exclude).
// It does not load global core.excludesfile (apply ownership must not depend
// on the operator's user git config).
//
// Returns nil when root is not a git worktree. The matcher is safe for
// concurrent Match calls after return.
func GitignoreUntracked(root string) func(absTarget string) bool {
	root = filepath.Clean(root)
	if root == "" || root == "." {
		return nil
	}
	gitRoot := findGitRoot(root)
	if gitRoot == "" {
		return nil
	}
	m := loadRepoIgnore(gitRoot)
	return func(absTarget string) bool {
		rel := RelToRoot(absTarget, gitRoot)
		if rel == absTarget || rel == "." || rel == "" {
			return false
		}
		return m.MatchPath(filepath.ToSlash(rel), false)
	}
}

// DropIgnored removes state entries for which ignore(target) is true.
// Returns how many keys were dropped. ignore nil is a no-op.
func DropIgnored(state *State, ignore func(string) bool) int {
	if state == nil || state.Files == nil || ignore == nil {
		return 0
	}
	n := 0
	for target := range state.Files {
		if ignore(target) {
			delete(state.Files, target)
			n++
		}
	}
	return n
}

func findGitRoot(start string) string {
	dir := filepath.Clean(start)
	for {
		if isGitWorkTree(dir) {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return ""
		}
		dir = parent
	}
}

func isGitWorkTree(root string) bool {
	st, err := os.Lstat(filepath.Join(root, ".git"))
	if err != nil {
		return false
	}
	return st.IsDir() || st.Mode().IsRegular()
}

func loadRepoIgnore(root string) *gitignore.Matcher {
	m := gitignore.New("")

	exclude := filepath.Join(root, ".git", "info", "exclude")
	if data, err := os.ReadFile(exclude); err == nil {
		m.AddPatterns(data, "")
	}
	rootGI := filepath.Join(root, ".gitignore")
	if data, err := os.ReadFile(rootGI); err == nil {
		m.AddPatterns(data, "")
	}

	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if path == root {
			return nil
		}
		rel, relErr := filepath.Rel(root, path)
		if relErr != nil {
			return nil
		}
		rel = filepath.ToSlash(rel)
		if d.IsDir() {
			if d.Name() == ".git" {
				return filepath.SkipDir
			}
			if m.MatchPath(rel, true) {
				return filepath.SkipDir
			}
			return nil
		}
		if d.Name() != ".gitignore" {
			return nil
		}
		dirRel := filepath.ToSlash(filepath.Dir(rel))
		if dirRel == "." {
			return nil
		}
		m.AddFromFile(path, dirRel)
		return nil
	})
	if err != nil {
		return m
	}
	return m
}
