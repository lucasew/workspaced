package backup

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	execdriver "workspaced/pkg/driver/exec"
	"workspaced/pkg/driver/notification"
)

var (
	// ErrGitRepoSyncNeedsSrcAndDst is returned when a git_repo_sync action is missing src or dst.
	ErrGitRepoSyncNeedsSrcAndDst = errors.New("git_repo_sync requires src and dst")
)

func init() {
	registerActionProvider[GitRepoSyncAction]("git_repo_sync")

}

type GitRepoSyncAction struct {
	backupActionBase
	Src string `json:"src"`
	Dst string `json:"dst"`
}

func (a GitRepoSyncAction) Run(ctx context.Context, _ *notification.Notification) error {
	if strings.TrimSpace(a.Src) == "" || strings.TrimSpace(a.Dst) == "" {
		return ErrGitRepoSyncNeedsSrcAndDst
	}
	if err := ensureRepoCloned(ctx, a.Src, a.Dst); err != nil {
		return err
	}
	if err := commitIfDirty(ctx, a.Src); err != nil {
		return err
	}
	if err := ensureRemoteURL(ctx, a.Src, BackupGitRemoteName, a.Dst); err != nil {
		return fmt.Errorf("failed to ensure remote %s for %s: %w", BackupGitRemoteName, a.Src, err)
	}
	branchCmd := execdriver.MustRun(ctx, "git", "-C", a.Src, "rev-parse", "--abbrev-ref", "HEAD")
	branchCmd.Stderr = os.Stderr
	branchOut, err := branchCmd.Output()
	if err != nil {
		return fmt.Errorf("failed to detect current branch for %s: %w", a.Src, err)
	}
	branch := strings.TrimSpace(string(branchOut))
	if branch == "" {
		return fmt.Errorf("empty current branch for %s", a.Src)
	}

	remoteBranchExists := runCommand(ctx, execdriver.MustRun(ctx, "git", "-C", a.Src, "ls-remote", "--exit-code", "--heads", BackupGitRemoteName, branch)) == nil
	if remoteBranchExists {
		if err := runCommand(ctx, execdriver.MustRun(ctx, "git", "-C", a.Src, "pull", "--rebase", BackupGitRemoteName, branch)); err != nil {
			_ = runCommand(ctx, execdriver.MustRun(ctx, "git", "-C", a.Src, "rebase", "--abort"))
			return fmt.Errorf("git pull --rebase failed for %s: %w", a.Src, err)
		}
	}
	if err := runCommand(ctx, execdriver.MustRun(ctx, "git", "-C", a.Src, "push", "-u", BackupGitRemoteName, branch)); err != nil {
		return fmt.Errorf("git push failed for %s: %w", a.Src, err)
	}
	return nil
}

func commitIfDirty(ctx context.Context, repoPath string) error {
	if err := runCommand(ctx, execdriver.MustRun(ctx, "git", "-C", repoPath, "add", "-A")); err != nil {
		return fmt.Errorf("git add failed for %s: %w", repoPath, err)
	}
	// Unborn HEAD (no commits yet): only commit when the index has staged paths.
	if !repoHasHEAD(ctx, repoPath) {
		statusCmd := execdriver.MustRun(ctx, "git", "-C", repoPath, "diff", "--cached", "--quiet")
		statusCmd.Stderr = os.Stderr
		if err := statusCmd.Run(); err == nil {
			return nil // nothing staged
		}
	} else if runCommand(ctx, execdriver.MustRun(ctx, "git", "-C", repoPath, "diff-index", "HEAD", "--exit-code")) == nil {
		return nil // clean vs HEAD
	}
	hostname, _ := os.Hostname()
	commitMsg := fmt.Sprintf("backup checkpoint %s", hostname)
	if err := runCommand(ctx, execdriver.MustRun(ctx, "git", "-C", repoPath, "commit", "-sm", commitMsg)); err != nil {
		return fmt.Errorf("git commit failed for %s: %w", repoPath, err)
	}
	return nil
}

func ensureRemoteURL(ctx context.Context, repoPath, remoteName, remoteURL string) error {
	getURLCmd := execdriver.MustRun(ctx, "git", "-C", repoPath, "remote", "get-url", remoteName)
	getURLCmd.Stderr = os.Stderr
	currentURLBytes, err := getURLCmd.Output()
	if err != nil {
		if err := runCommand(ctx, execdriver.MustRun(ctx, "git", "-C", repoPath, "remote", "add", remoteName, remoteURL)); err != nil {
			return err
		}
		return nil
	}
	currentURL := strings.TrimSpace(string(currentURLBytes))
	if currentURL == remoteURL {
		return nil
	}
	return runCommand(ctx, execdriver.MustRun(ctx, "git", "-C", repoPath, "remote", "set-url", remoteName, remoteURL))
}

// ensureRepoCloned makes sure repoPath is a real clone of remoteURL (checkout with commits),
// not an empty git init + remote add/fetch shell. Initial setup is always `git clone`.
func ensureRepoCloned(ctx context.Context, repoPath, remoteURL string) error {
	gitDir := filepath.Join(repoPath, ".git")
	if _, err := os.Stat(gitDir); err == nil {
		if repoHasHEAD(ctx, repoPath) {
			return nil
		}
		// Unborn HEAD: only safe to replace with clone when there is no local work.
		onlyGitMeta, err := dirOnlyDotGit(repoPath)
		if err != nil {
			return err
		}
		if !onlyGitMeta {
			return fmt.Errorf("repo path has .git but no commits and is not empty enough to re-clone: %s", repoPath)
		}
		if err := os.RemoveAll(repoPath); err != nil {
			return fmt.Errorf("failed to remove empty init at %s before clone: %w", repoPath, err)
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("failed to inspect %s: %w", gitDir, err)
	}

	if err := os.MkdirAll(filepath.Dir(repoPath), 0755); err != nil {
		return fmt.Errorf("failed to create parent dir for %s: %w", repoPath, err)
	}

	if info, err := os.Stat(repoPath); err == nil {
		if !info.IsDir() {
			return fmt.Errorf("repo path exists and is not a directory: %s", repoPath)
		}
		entries, readErr := os.ReadDir(repoPath)
		if readErr != nil {
			return fmt.Errorf("failed to inspect repo path %s: %w", repoPath, readErr)
		}
		if len(entries) > 0 {
			return fmt.Errorf("repo path exists but is not a git repo and is not empty: %s", repoPath)
		}
		// Empty dir: remove so `git clone remote path` can create it.
		if err := os.Remove(repoPath); err != nil {
			return fmt.Errorf("failed to remove empty dir %s before clone: %w", repoPath, err)
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("failed to inspect repo path %s: %w", repoPath, err)
	}

	if err := runCommand(ctx, execdriver.MustRun(ctx, "git", "clone", remoteURL, repoPath)); err != nil {
		return fmt.Errorf("git clone failed for %s from %s: %w", repoPath, remoteURL, err)
	}
	// Some remotes advertise a broken HEAD (e.g. still points at master while only main exists).
	// git clone then leaves an unborn branch; pick a real remote branch to check out.
	if !repoHasHEAD(ctx, repoPath) {
		if err := checkoutRemoteDefaultBranch(ctx, repoPath); err != nil {
			return fmt.Errorf("clone at %s has no HEAD and fallback checkout failed: %w", repoPath, err)
		}
	}
	return nil
}

func repoHasHEAD(ctx context.Context, repoPath string) bool {
	cmd := execdriver.MustRun(ctx, "git", "-C", repoPath, "rev-parse", "--verify", "HEAD")
	cmd.Stderr = os.Stderr
	return cmd.Run() == nil
}

// checkoutRemoteDefaultBranch checks out origin's default or first remote branch after a clone
// that could not materialize HEAD (broken remote HEAD symref is common on bare rsync remotes).
func checkoutRemoteDefaultBranch(ctx context.Context, repoPath string) error {
	// Prefer origin/HEAD when it resolves to a concrete branch.
	symCmd := execdriver.MustRun(ctx, "git", "-C", repoPath, "symbolic-ref", "--quiet", "refs/remotes/origin/HEAD")
	symCmd.Stderr = os.Stderr
	if out, err := symCmd.Output(); err == nil {
		ref := strings.TrimSpace(string(out)) // e.g. refs/remotes/origin/main
		if branch, ok := strings.CutPrefix(ref, "refs/remotes/origin/"); ok && branch != "" {
			if err := runCommand(ctx, execdriver.MustRun(ctx, "git", "-C", repoPath, "checkout", "-B", branch, "origin/"+branch)); err == nil {
				return nil
			}
		}
	}

	listCmd := execdriver.MustRun(ctx, "git", "-C", repoPath, "for-each-ref", "--format=%(refname:short)", "refs/remotes/origin")
	listCmd.Stderr = os.Stderr
	out, err := listCmd.Output()
	if err != nil {
		return fmt.Errorf("list remote branches: %w", err)
	}
	var candidates []string
	for _, line := range strings.Split(string(out), "\n") {
		name := strings.TrimSpace(line)
		if name == "" || name == "origin" || name == "origin/HEAD" {
			continue
		}
		branch, ok := strings.CutPrefix(name, "origin/")
		if !ok || branch == "" {
			continue
		}
		// Prefer main/master if present.
		if branch == "main" || branch == "master" {
			candidates = append([]string{branch}, candidates...)
		} else {
			candidates = append(candidates, branch)
		}
	}
	// Dedupe while preserving preference order (main/master first).
	seen := map[string]bool{}
	for _, branch := range candidates {
		if seen[branch] {
			continue
		}
		seen[branch] = true
		if err := runCommand(ctx, execdriver.MustRun(ctx, "git", "-C", repoPath, "checkout", "-B", branch, "origin/"+branch)); err == nil {
			return nil
		}
	}
	return errors.New("no usable origin/* branch to check out")
}

// dirOnlyDotGit reports whether repoPath contains only a .git entry (safe to wipe for re-clone).
func dirOnlyDotGit(repoPath string) (bool, error) {
	entries, err := os.ReadDir(repoPath)
	if err != nil {
		return false, fmt.Errorf("failed to inspect repo path %s: %w", repoPath, err)
	}
	if len(entries) == 0 {
		return true, nil
	}
	if len(entries) == 1 && entries[0].Name() == ".git" {
		return true, nil
	}
	return false, nil
}

func runCommand(_ context.Context, cmd *exec.Cmd) error {
	cmd.Stdout = os.Stderr
	cmd.Stderr = os.Stderr
	return cmd.Run()
}
