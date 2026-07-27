package open

import (
	"context"
	"fmt"
	"os"

	"github.com/lucasew/workspaced/internal/miseutil"
	execdriver "github.com/lucasew/workspaced/pkg/driver/exec"
	"github.com/lucasew/workspaced/pkg/logging"

	"github.com/spf13/cobra"
)

// ensureMise resolves mise through the home lazy tool path and refreshes
// the ~/.local/bin wrapper that re-enters this command.
func ensureMise(ctx context.Context) (string, error) {
	logger := logging.GetLogger(ctx)
	misePath, err := miseutil.Ensure(ctx)
	if err != nil {
		return "", err
	}

	if err := miseutil.EnsureLocalBinWrapper(ctx, ""); err != nil {
		logger.Warn("failed to create mise wrapper", "error", err)
	}

	return misePath, nil
}

func miseCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:                "mise [args...]",
		Short:              "Run mise (installs as a lazy tool if needed)",
		DisableFlagParsing: true,
		Long: `Run mise resolved through workspaced's standard tool path.

mise is a catalog tool (registry:mise / github:jdx/mise) and a home lazy tool
named "mise". The first invocation installs it into the tool store and pins the
version in the home/dotfiles workspaced.lock.json (not in codebase locks).

Examples:
  workspaced open mise version
  workspaced open mise install node@20
  workspaced open mise use -g python@3.11
  workspaced open lazy --home mise -- version`,
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()

			misePath, err := ensureMise(ctx)
			if err != nil {
				return err
			}

			miseCmd, err := execdriver.Run(ctx, misePath, args...)
			if err != nil {
				return fmt.Errorf("create command: %w", err)
			}

			miseCmd.Stdin = os.Stdin
			miseCmd.Stdout = os.Stdout
			miseCmd.Stderr = os.Stderr

			return miseCmd.Run()
		},
	}
	return cmd
}
