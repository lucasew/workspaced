package git

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	execdriver "github.com/lucasew/workspaced/pkg/driver/exec"
	"github.com/lucasew/workspaced/pkg/driver/notification"
	"github.com/lucasew/workspaced/pkg/logging"
)

// QuickSync walks repoDir for git checkouts and SyncRepo's each one.
// Callers load workspaced quicksync.repo_dir (or pass any directory of repos).
// Does not import configcue so lower packages can use GetRoot without a cycle.
func QuickSync(ctx context.Context, repoDir string) error {
	logger := logging.GetLogger(ctx)
	entries, err := os.ReadDir(repoDir)
	if err != nil {
		return fmt.Errorf("read repo dir %s: %w", repoDir, err)
	}

	var repos []string
	for _, entry := range entries {
		if entry.IsDir() {
			repoPath := filepath.Join(repoDir, entry.Name())
			if _, err := os.Stat(filepath.Join(repoPath, ".git")); err == nil {
				repos = append(repos, entry.Name())
			}
		}
	}

	total := len(repos)
	if total == 0 {
		return nil
	}

	n := &notification.Notification{
		Title: "Git Sync",
		Icon:  "git",
	}

	for i, repoName := range repos {
		if err := ctx.Err(); err != nil {
			return err
		}

		repoPath := filepath.Join(repoDir, repoName)
		n.Message = fmt.Sprintf("Syncing %s...", repoName)
		n.Progress = float64(i) / float64(total)
		logging.ReportError(ctx, notification.Notify(ctx, n))

		logger.Info("syncing repository", "repo", repoName)
		if err := SyncRepo(ctx, repoPath); err != nil {
			logger.Error("failed to sync repo", "repo", repoName, "error", err)
			errN := &notification.Notification{
				Title:   "Sync Failed",
				Message: fmt.Sprintf("Conflict or error in %s. Manual intervention required.", repoName),
				Urgency: "critical",
				Icon:    "dialog-warning",
			}
			logging.ReportError(ctx, notification.Notify(ctx, errN))
		}
	}

	n.Message = "Sync completed."
	n.Progress = 1.0
	logging.ReportError(ctx, notification.Notify(ctx, n))

	return nil
}

func SyncRepo(ctx context.Context, path string) error {
	hostname, _ := os.Hostname()
	logger := logging.GetLogger(ctx)

	logger.Info("git add", "path", path)
	if err := execdriver.MustRun(ctx, "git", "-C", path, "add", "-A").Run(); err != nil {
		return fmt.Errorf("git add failed: %w", err)
	}

	// Check if there are changes to commit
	if err := execdriver.MustRun(ctx, "git", "-C", path, "diff-index", "HEAD", "--exit-code").Run(); err != nil {
		commitMsg := fmt.Sprintf("backup checkpoint %s", hostname)
		logger.Info("git commit", "path", path, "msg", commitMsg)
		if err := execdriver.MustRun(ctx, "git", "-C", path, "commit", "-sm", commitMsg).Run(); err != nil {
			return fmt.Errorf("git commit failed: %w", err)
		}
	}

	logger.Info("git pull --rebase", "path", path)
	if err := execdriver.MustRun(ctx, "git", "-C", path, "pull", "--rebase").Run(); err != nil {
		if abortErr := execdriver.MustRun(ctx, "git", "-C", path, "rebase", "--abort").Run(); abortErr != nil {
			logging.ReportError(ctx, abortErr, "op", "git rebase --abort", "path", path)
		}
		return fmt.Errorf("git pull rebase failed (conflict?): %w", err)
	}

	logger.Info("git push", "path", path)
	if err := execdriver.MustRun(ctx, "git", "-C", path, "push").Run(); err != nil {
		return fmt.Errorf("git push failed: %w", err)
	}

	return nil
}

// GetRoot returns the root directory of the git repository containing path.
func GetRoot(ctx context.Context, path string) (string, error) {
	if _, err := os.Stat(path); err != nil {
		return "", err
	}

	cmd := execdriver.MustRun(ctx, "git", "-C", path, "rev-parse", "--show-toplevel")
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("git root: %w", err)
	}
	return strings.TrimSpace(string(out)), nil
}
