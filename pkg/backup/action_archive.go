package backup

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	execdriver "workspaced/pkg/driver/exec"
	"workspaced/pkg/driver/notification"
)

func init() {
	registerActionProvider[ArchiveAction]("archive")
}

type ArchiveAction struct {
	backupActionBase
	InputDir string `json:"input_dir"`
	Output   string `json:"output"`
	Format   string `json:"format"`
}

func (action ArchiveAction) Run(ctx context.Context, _ *notification.Notification) error {
	if strings.TrimSpace(action.InputDir) == "" || strings.TrimSpace(action.Output) == "" {
		return fmt.Errorf("archive action requires input_dir and output")
	}
	if action.Format != "tar" {
		return fmt.Errorf("unsupported archive format: %s", action.Format)
	}
	parent := filepath.Dir(action.InputDir)
	base := filepath.Base(action.InputDir)
	cmd := execdriver.MustRun(ctx, "tar", "-cvf", action.Output, "-C", parent, base)
	if verboseOutput(ctx) {
		cmd.Stderr = os.Stderr
	}
	return cmd.Run()
}
