package backup

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"

	execdriver "workspaced/pkg/driver/exec"
	"workspaced/pkg/driver/notification"
)

var (
	// ErrGitRepoSyncNeedsSrcAndDst is returned when a git_repo_sync action is missing src or dst.
	ErrGitRepoSyncNeedsSrcAndDst = errors.New("git_repo_sync requires src and dst")
)

func init() {
	registerAction[GitRepoSyncAction]("git_repo_sync")
}

// GitRepoSyncAction backs up a local git working tree by committing changes and pushing to Dst.
// Src is the local path; Dst is the remote URL (also used as the workspaced remote).
type GitRepoSyncAction struct {
	backupActionBase
	Src string `json:"src"`
	Dst string `json:"dst"`
}

func (a GitRepoSyncAction) Run(ctx context.Context, _ *notification.Notification) error {
	if strings.TrimSpace(a.Src) == "" || strings.TrimSpace(a.Dst) == "" {
		return ErrGitRepoSyncNeedsSrcAndDst
	}

	// Initial sync: always clone. Replace empty init shells; never init+fetch in place.
	if !a.hasGitDir() {
		if err := a.prepareEmptyPathForClone(); err != nil {
			return err
		}
		if err := a.clone(ctx); err != nil {
			return err
		}
	} else if !a.hasHEAD(ctx) {
		if !a.dirOnlyDotGit() {
			return fmt.Errorf("repo path has .git but no commits and is not empty enough to re-clone: %s", a.Src)
		}
		if err := os.RemoveAll(a.Src); err != nil {
			return fmt.Errorf("failed to remove empty init at %s before clone: %w", a.Src, err)
		}
		if err := a.clone(ctx); err != nil {
			return err
		}
	}

	// Broken remote HEAD (e.g. master advertised, only main exists) leaves clone without checkout.
	if !a.hasHEAD(ctx) {
		if err := a.checkoutRemoteDefaultBranch(ctx); err != nil {
			return fmt.Errorf("repo at %s has no HEAD and fallback checkout failed: %w", a.Src, err)
		}
	}

	if err := a.commitIfDirty(ctx); err != nil {
		return err
	}
	if err := a.ensureRemoteURL(ctx, BackupGitRemoteName, a.Dst); err != nil {
		return fmt.Errorf("failed to ensure remote %s for %s: %w", BackupGitRemoteName, a.Src, err)
	}

	branch, err := a.currentBranch(ctx)
	if err != nil {
		return err
	}
	if a.remoteHasBranch(ctx, BackupGitRemoteName, branch) {
		if err := a.pullRebase(ctx, BackupGitRemoteName, branch); err != nil {
			return err
		}
	}
	if err := a.push(ctx, BackupGitRemoteName, branch); err != nil {
		return err
	}
	return nil
}

func (a GitRepoSyncAction) cmd(ctx context.Context, args ...string) *exec.Cmd {
	argv := append([]string{"-C", a.Src}, args...)
	return execdriver.MustRun(ctx, "git", argv...)
}

func (a GitRepoSyncAction) run(ctx context.Context, args ...string) error {
	cmd := a.cmd(ctx, args...)
	cmd.Stdout = os.Stderr
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func (a GitRepoSyncAction) output(ctx context.Context, args ...string) ([]byte, error) {
	cmd := a.cmd(ctx, args...)
	cmd.Stderr = os.Stderr
	return cmd.Output()
}

func (a GitRepoSyncAction) hasGitDir() bool {
	_, err := os.Stat(filepath.Join(a.Src, ".git"))
	return err == nil
}

func (a GitRepoSyncAction) hasHEAD(ctx context.Context) bool {
	return a.run(ctx, "rev-parse", "--verify", "HEAD") == nil
}

// dirOnlyDotGit is true when Src has no local work beyond git metadata (safe to wipe for re-clone).
func (a GitRepoSyncAction) dirOnlyDotGit() bool {
	entries, err := os.ReadDir(a.Src)
	if err != nil {
		return false
	}
	if len(entries) == 0 {
		return true
	}
	return len(entries) == 1 && entries[0].Name() == ".git"
}

func (a GitRepoSyncAction) prepareEmptyPathForClone() error {
	if err := os.MkdirAll(filepath.Dir(a.Src), 0755); err != nil {
		return fmt.Errorf("failed to create parent dir for %s: %w", a.Src, err)
	}
	info, err := os.Stat(a.Src)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("failed to inspect repo path %s: %w", a.Src, err)
	}
	if !info.IsDir() {
		return fmt.Errorf("repo path exists and is not a directory: %s", a.Src)
	}
	entries, err := os.ReadDir(a.Src)
	if err != nil {
		return fmt.Errorf("failed to inspect repo path %s: %w", a.Src, err)
	}
	if len(entries) > 0 {
		return fmt.Errorf("repo path exists but is not a git repo and is not empty: %s", a.Src)
	}
	// Empty dir: remove so `git clone remote path` can create it.
	if err := os.Remove(a.Src); err != nil {
		return fmt.Errorf("failed to remove empty dir %s before clone: %w", a.Src, err)
	}
	return nil
}

func (a GitRepoSyncAction) clone(ctx context.Context) error {
	cmd := execdriver.MustRun(ctx, "git", "clone", a.Dst, a.Src)
	cmd.Stdout = os.Stderr
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("git clone failed for %s from %s: %w", a.Src, a.Dst, err)
	}
	return nil
}

func (a GitRepoSyncAction) currentBranch(ctx context.Context) (string, error) {
	out, err := a.output(ctx, "rev-parse", "--abbrev-ref", "HEAD")
	if err != nil {
		return "", fmt.Errorf("failed to detect current branch for %s: %w", a.Src, err)
	}
	branch := strings.TrimSpace(string(out))
	if branch == "" {
		return "", fmt.Errorf("empty current branch for %s", a.Src)
	}
	return branch, nil
}

func (a GitRepoSyncAction) commitIfDirty(ctx context.Context) error {
	if err := a.run(ctx, "add", "-A"); err != nil {
		return fmt.Errorf("git add failed for %s: %w", a.Src, err)
	}
	// Unborn HEAD: only commit when the index has staged paths.
	if !a.hasHEAD(ctx) {
		if a.run(ctx, "diff", "--cached", "--quiet") == nil {
			return nil
		}
	} else if a.run(ctx, "diff-index", "HEAD", "--exit-code") == nil {
		return nil
	}
	hostname, _ := os.Hostname()
	commitMsg := fmt.Sprintf("backup checkpoint %s", hostname)
	if err := a.run(ctx, "commit", "-sm", commitMsg); err != nil {
		return fmt.Errorf("git commit failed for %s: %w", a.Src, err)
	}
	return nil
}

func (a GitRepoSyncAction) ensureRemoteURL(ctx context.Context, remoteName, remoteURL string) error {
	currentURLBytes, err := a.output(ctx, "remote", "get-url", remoteName)
	if err != nil {
		return a.run(ctx, "remote", "add", remoteName, remoteURL)
	}
	if strings.TrimSpace(string(currentURLBytes)) == remoteURL {
		return nil
	}
	return a.run(ctx, "remote", "set-url", remoteName, remoteURL)
}

func (a GitRepoSyncAction) remoteHasBranch(ctx context.Context, remoteName, branch string) bool {
	return a.run(ctx, "ls-remote", "--exit-code", "--heads", remoteName, branch) == nil
}

func (a GitRepoSyncAction) pullRebase(ctx context.Context, remoteName, branch string) error {
	if err := a.run(ctx, "pull", "--rebase", remoteName, branch); err != nil {
		_ = a.run(ctx, "rebase", "--abort")
		return fmt.Errorf("git pull --rebase failed for %s: %w", a.Src, err)
	}
	return nil
}

func (a GitRepoSyncAction) push(ctx context.Context, remoteName, branch string) error {
	if err := a.run(ctx, "push", "-u", remoteName, branch); err != nil {
		return fmt.Errorf("git push failed for %s: %w", a.Src, err)
	}
	return nil
}

// checkoutRemoteDefaultBranch checks out origin's default or first remote branch after a clone
// that could not materialize HEAD (broken remote HEAD symref is common on bare rsync remotes).
func (a GitRepoSyncAction) checkoutRemoteDefaultBranch(ctx context.Context) error {
	if out, err := a.output(ctx, "symbolic-ref", "--quiet", "refs/remotes/origin/HEAD"); err == nil {
		ref := strings.TrimSpace(string(out))
		if branch, ok := strings.CutPrefix(ref, "refs/remotes/origin/"); ok && branch != "" {
			if err := a.run(ctx, "checkout", "-B", branch, "origin/"+branch); err == nil {
				return nil
			}
		}
	}

	out, err := a.output(ctx, "for-each-ref", "--format=%(refname:short)", "refs/remotes/origin")
	if err != nil {
		return fmt.Errorf("list remote branches: %w", err)
	}
	var preferred, others []string
	seen := map[string]bool{}
	for _, line := range strings.Split(string(out), "\n") {
		name := strings.TrimSpace(line)
		if name == "" || name == "origin" || name == "origin/HEAD" {
			continue
		}
		branch, ok := strings.CutPrefix(name, "origin/")
		if !ok || branch == "" || seen[branch] {
			continue
		}
		seen[branch] = true
		if branch == "main" || branch == "master" {
			preferred = append(preferred, branch)
		} else {
			others = append(others, branch)
		}
	}
	// Historical prepend tried defaults last-first (e.g. master before main).
	slices.Reverse(preferred)
	candidates := append(preferred, others...)
	for _, branch := range candidates {
		if err := a.run(ctx, "checkout", "-B", branch, "origin/"+branch); err == nil {
			return nil
		}
	}
	return errors.New("no usable origin/* branch to check out")
}
