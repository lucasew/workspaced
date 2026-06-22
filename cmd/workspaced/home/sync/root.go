package sync

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	execdriver "workspaced/pkg/driver/exec"
	envdriver "workspaced/pkg/driver/env"
	"workspaced/pkg/logging"
)

func GetCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "sync",
		Short: "Pull dotfiles changes and apply them",
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			root, err := envdriver.GetDotfilesRoot(ctx)
			if err != nil {
				return fmt.Errorf("failed to get dotfiles root: %w", err)
			}

			logger := logging.GetLogger(ctx)
			logger.Info("==> Pulling dotfiles changes...")
			pullCmd := execdriver.MustRun(ctx, "git", "-C", root, "pull")
			pullCmd.Stdout = os.Stdout
			pullCmd.Stderr = os.Stderr
			if err := pullCmd.Run(); err != nil {
				return fmt.Errorf("git pull failed: %w", err)
			}

			c := execdriver.MustRun(ctx, "workspaced", "self-update")
			c.Stdout = os.Stdout
			c.Stderr = os.Stderr
			if err := c.Run(); err != nil {
				return fmt.Errorf("command failed: %w", err)
			}

			return nil
		},
	}
}
