package open

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	execdriver "workspaced/pkg/driver/exec"

	"github.com/spf13/cobra"
)

func execCommand() *cobra.Command {
	var pathDirs []string

	cmd := &cobra.Command{
		Use:   "exec <command> [args...]",
		Short: "Execute a command using the platform-appropriate exec driver",
		Long: `Execute a command using the platform-appropriate exec driver.

This runs client-side and is useful on Termux where some commands may fail
due to SIGSYS errors. The exec driver handles platform-specific quirks automatically.

Examples:
  workspaced open exec git status
  workspaced open exec ls -la /data
  workspaced open exec -- command --with-flags
  workspaced open exec --path /custom/bin -- mycommand`,
		Args: cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()

			// Create command using driver
			command, err := execdriver.Run(ctx, args[0], args[1:]...)
			if err != nil {
				return fmt.Errorf("failed to create command: %w", err)
			}

			// Modify PATH if requested
			if len(pathDirs) > 0 {
				// Get current environment or create new one
				env := os.Environ()
				if command.Env != nil {
					env = command.Env
				}

				// Find and modify PATH
				pathModified := false
				for i, envVar := range env {
					if strings.HasPrefix(envVar, "PATH=") {
						currentPath := strings.TrimPrefix(envVar, "PATH=")

						// Prepend new paths
						newPaths := make([]string, 0, len(pathDirs)+1)
						for _, dir := range pathDirs {
							absDir, err := filepath.Abs(dir)
							if err != nil {
								return fmt.Errorf("invalid path %q: %w", dir, err)
							}
							newPaths = append(newPaths, absDir)
						}
						newPaths = append(newPaths, currentPath)

						env[i] = "PATH=" + strings.Join(newPaths, string(os.PathListSeparator))
						pathModified = true
						break
					}
				}

				// If PATH wasn't found, add it
				if !pathModified {
					newPaths := make([]string, 0, len(pathDirs))
					for _, dir := range pathDirs {
						absDir, err := filepath.Abs(dir)
						if err != nil {
							return fmt.Errorf("invalid path %q: %w", dir, err)
						}
						newPaths = append(newPaths, absDir)
					}
					env = append(env, "PATH="+strings.Join(newPaths, string(os.PathListSeparator)))
				}

				command.Env = env
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

	cmd.Flags().StringSliceVarP(&pathDirs, "path", "p", nil, "Prepend directories to PATH (can be specified multiple times)")

	// Stop parsing flags after the first positional argument
	// This allows the executed command to have its own flags without conflict
	cmd.Flags().SetInterspersed(false)

	return cmd
}
