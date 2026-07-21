package open

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/lucasew/workspaced/internal/miseutil"
	envdriver "github.com/lucasew/workspaced/pkg/driver/env"
	execdriver "github.com/lucasew/workspaced/pkg/driver/exec"
	"github.com/lucasew/workspaced/pkg/driver/shim/bash"
	"github.com/lucasew/workspaced/pkg/logging"

	"github.com/spf13/cobra"
)

// ensureMise checks if mise is installed and installs it if needed, then ensures the ~/.local/bin wrapper.
func ensureMise(ctx context.Context) (string, error) {
	logger := logging.GetLogger(ctx)
	misePath, err := miseutil.Ensure(ctx)
	if err != nil {
		return "", err
	}

	if err := ensureMiseWrapper(ctx, misePath); err != nil {
		logger.Warn("failed to create mise wrapper", "error", err)
	}

	return misePath, nil
}

// ensureMiseWrapper creates a wrapper script in ~/.local/bin/mise
func ensureMiseWrapper(ctx context.Context, misePath string) error {
	logger := logging.GetLogger(ctx)
	home, err := envdriver.ResolveHomeDir()
	if err != nil {
		return fmt.Errorf("get home directory: %w", err)
	}

	wrapperDir := filepath.Join(home, ".local", "bin")
	wrapperPath := filepath.Join(wrapperDir, "mise")

	workspacedBin := filepath.Join(home, ".local", "share", "workspaced", "bin", "workspaced")
	shell := bash.GetShell(ctx)
	expectedContent := fmt.Sprintf("#!%s\nexec -a \"$0\" %s open mise \"$@\"\n", shell, workspacedBin)

	if content, err := os.ReadFile(wrapperPath); err == nil && string(content) == expectedContent {
		return nil
	}

	if err := os.MkdirAll(wrapperDir, 0755); err != nil {
		return fmt.Errorf("create wrapper directory: %w", err)
	}

	if err := os.WriteFile(wrapperPath, []byte(expectedContent), 0755); err != nil {
		return fmt.Errorf("write wrapper: %w", err)
	}

	logger.Info("created mise wrapper", "path", wrapperPath)
	return nil
}

func miseCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:                "mise [args...]",
		Short:              "Run mise (installs automatically if needed)",
		DisableFlagParsing: true,
		Long: `Run mise using a custom installation path.

This command ensures mise is installed in a consistent location.

Installation path priority:
  1. MISE_INSTALL_PATH environment variable
  2. ~/.local/share/workspaced/bin/mise (default)

If mise is not found, it will be automatically installed using the official
installer from https://mise.run/

Examples:
  workspaced open mise --version
  workspaced open mise install node@20
  workspaced open mise use -g python@3.11
  MISE_INSTALL_PATH=/custom/path/mise workspaced open mise --version`,
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
