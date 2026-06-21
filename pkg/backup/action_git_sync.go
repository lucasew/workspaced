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
	hostname, _ := os.Hostname()
	if err := runCommand(ctx, execdriver.MustRun(ctx, "git", "-C", a.Src, "add", "-A")); err != nil {
		return fmt.Errorf("git add failed for %s: %w", a.Src, err)
	}
	if err := runCommand(ctx, execdriver.MustRun(ctx, "git", "-C", a.Src, "diff-index", "HEAD", "--exit-code")); err != nil {
		commitMsg := fmt.Sprintf("backup checkpoint %s", hostname)
		if err := runCommand(ctx, execdriver.MustRun(ctx, "git", "-C", a.Src, "commit", "-sm", commitMsg)); err != nil {
			return fmt.Errorf("git commit failed for %s: %w", a.Src, err)
		}
	}
	if err := ensureRemoteURL(ctx, a.Src, BackupGitRemoteName, a.Dst); err != nil {
		return fmt.Errorf("failed to ensure remote %s for %s: %w", BackupGitRemoteName, a.Src, err)
	}
	branchOut, err := execdriver.MustRun(ctx, "git", "-C", a.Src, "rev-parse", "--abbrev-ref", "HEAD").Output()
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

func ensureRemoteURL(ctx context.Context, repoPath, remoteName, remoteURL string) error {
	currentURLBytes, err := execdriver.MustRun(ctx, "git", "-C", repoPath, "remote", "get-url", remoteName).Output()
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

func ensureRepoCloned(ctx context.Context, repoPath, remoteURL string) error {
	if _, err := os.Stat(filepath.Join(repoPath, ".git")); err == nil {
		return nil
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
	} else if !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("failed to inspect repo path %s: %w", repoPath, err)
	}

	if err := runCommand(ctx, execdriver.MustRun(ctx, "git", "clone", remoteURL, repoPath)); err != nil {
		return fmt.Errorf("git clone failed for %s from %s: %w", repoPath, remoteURL, err)
	}
	return nil
}

func runCommand(_ context.Context, cmd *exec.Cmd) error {
	cmd.Stderr = os.Stderr
	return cmd.Run()
}
