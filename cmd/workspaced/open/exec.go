package open

import (
	"fmt"
	"os"
	execdriver "workspaced/pkg/driver/exec"

	"github.com/spf13/cobra"
)

func execCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "exec <command> [args...]",
		Short: "Execute a command using the platform-appropriate exec driver",
		Long: `Execute a command using the platform-appropriate exec driver.

This runs client-side and is useful on Termux where some commands may fail
due to SIGSYS errors. The exec driver handles platform-specific quirks automatically.

Examples:
  workspaced open exec git status
  workspaced open exec ls -la /data
  workspaced open exec -- command --with-flags`,
		Args: cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()

			// Create command using driver
			command, err := execdriver.Run(ctx, args[0], args[1:]...)
			if err != nil {
				return fmt.Errorf("failed to create command: %w", err)
			}

			// Connect stdio
			command.Stdin = os.Stdin
			command.Stdout = os.Stdout
			command.Stderr = os.Stderr

			// Run and return exit code
			if err := command.Run(); err != nil {
				return err
			}

			return nil
		},
	}
}
