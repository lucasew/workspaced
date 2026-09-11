package sync

import (
	"context"
	"fmt"
	"os"

	envdriver "github.com/lucasew/workspaced/pkg/driver/env"
	execdriver "github.com/lucasew/workspaced/pkg/driver/exec"
	"github.com/lucasew/workspaced/pkg/logging"
)

type Command struct{}

func (Command) Description() string {
	return "Pull dotfiles changes and apply them"
}

func (c *Command) Run(ctx context.Context) error {
	root, err := envdriver.GetDotfilesRoot(ctx)
	if err != nil {
		return fmt.Errorf("get dotfiles root: %w", err)
	}

	logger := logging.GetLogger(ctx)
	logger.Info("==> Pulling dotfiles changes...")
	pullCmd := execdriver.MustRun(ctx, "git", "-C", root, "pull")
	pullCmd.Stdout = os.Stdout
	pullCmd.Stderr = os.Stderr
	if err := pullCmd.Run(); err != nil {
		return fmt.Errorf("git pull failed: %w", err)
	}

	cmd := execdriver.MustRun(ctx, "workspaced", "self-update")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("command failed: %w", err)
	}

	return nil
}
