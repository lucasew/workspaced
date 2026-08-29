package open

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	execdriver "github.com/lucasew/workspaced/pkg/driver/exec"
	"github.com/lucasew/workspaced/pkg/taskgroup"

	"github.com/spf13/cobra"
)

func execCommand() *cobra.Command {
	var pathDirs []string

	cmd := &cobra.Command{
		Use:   "exec <command> [args...]",
		Short: "Execute a command using the platform-appropriate exec driver",
		Long: `Execute a command using the platform-appropriate exec driver.

Examples:
  workspaced open exec git status
  workspaced open exec ls -la /data
  workspaced open exec -- command --with-flags
  workspaced open exec --path /custom/bin -- mycommand`,
		Args: cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()

			command, err := execdriver.Run(ctx, args[0], args[1:]...)
			if err != nil {
				return fmt.Errorf("create command: %w", err)
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
					if after, ok := strings.CutPrefix(envVar, "PATH="); ok {
						currentPath := after

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

			// Defer run until session Close tears down UI/stderr patches so the
			// child inherits real stdio (even when no tasks were scheduled).
			var theCmd = command
			taskgroup.MustSessionFrom(ctx).AfterWait(func() error {
				theCmd.Stdin = os.Stdin
				theCmd.Stdout = os.Stdout
				theCmd.Stderr = os.Stderr
				return theCmd.Run()
			})
			return nil
		},
	}

	cmd.Flags().StringSliceVarP(&pathDirs, "path", "p", nil, "Prepend directories to PATH (can be specified multiple times)")

	// Stop parsing flags after the first positional argument
	// This allows the executed command to have its own flags without conflict
	cmd.Flags().SetInterspersed(false)

	return cmd
}
