package backup

import (
	"context"
	"fmt"
	"os"
	"strings"
	execdriver "workspaced/pkg/driver/exec"
	"workspaced/pkg/driver/notification"
)

func init() {
	registerActionProvider[GitRepoSyncAction]("git_sync")

}

type GitRepoSyncAction struct {
	backupActionBase
	Src string `json:"src"`
	Dst string `json:"dst"`
}

func (a GitRepoSyncAction) Run(ctx context.Context, _ *notification.Notification) error {
	if strings.TrimSpace(a.Src) == "" || strings.TrimSpace(a.Dst) == "" {
		return fmt.Errorf("git_repo_sync requires src and dst")
	}
	remoteName := "workspaced_backup"
	hostname, _ := os.Hostname()
	if err := execdriver.MustRun(ctx, "git", "-C", a.Src, "add", "-A").Run(); err != nil {
		return fmt.Errorf("git add failed for %s: %w", a.Src, err)
	}
	if err := execdriver.MustRun(ctx, "git", "-C", a.Src, "diff-index", "HEAD", "--exit-code").Run(); err != nil {
		commitMsg := fmt.Sprintf("backup checkpoint %s", hostname)
		if err := execdriver.MustRun(ctx, "git", "-C", a.Src, "commit", "-sm", commitMsg).Run(); err != nil {
			return fmt.Errorf("git commit failed for %s: %w", a.Src, err)
		}
	}
	if err := execdriver.MustRun(ctx, "git", "-C", a.Src, "remote", "add", remoteName, a.Dst).Run(); err != nil {
		_ = execdriver.MustRun(ctx, "git", "-C", a.Src, "remote", "set-url", remoteName, a.Dst).Run()
	}
	branchOut, err := execdriver.MustRun(ctx, "git", "-C", a.Src, "rev-parse", "--abbrev-ref", "HEAD").Output()
	if err != nil {
		return fmt.Errorf("failed to detect current branch for %s: %w", a.Src, err)
	}
	branch := strings.TrimSpace(string(branchOut))
	if branch == "" {
		return fmt.Errorf("empty current branch for %s", a.Src)
	}
	if err := execdriver.MustRun(ctx, "git", "-C", a.Src, "pull", "--rebase", remoteName, branch).Run(); err != nil {
		_ = execdriver.MustRun(ctx, "git", "-C", a.Src, "rebase", "--abort").Run()
		return fmt.Errorf("git pull --rebase failed for %s: %w", a.Src, err)
	}
	if err := execdriver.MustRun(ctx, "git", "-C", a.Src, "push", remoteName, branch).Run(); err != nil {
		return fmt.Errorf("git push failed for %s: %w", a.Src, err)
	}
	return nil
}
