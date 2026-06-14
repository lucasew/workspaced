package tool

import (
	"context"
	"fmt"
	"os"
	"os/exec"

	execdriver "workspaced/pkg/driver/exec"
	"workspaced/pkg/taskgroup"
	"workspaced/pkg/tool"

	"github.com/spf13/cobra"
)

func init() {
	Registry.FromGetter(
		func() *cobra.Command {
			return &cobra.Command{
				Use:   "with <tool-spec> -- <command> [args...]",
				Short: "Execute a command with a specific tool version",
				Long: `Execute a command with a specific tool version.

If the tool is not installed, it will be installed automatically.

Tool spec format:
  provider:package@version  (full spec)
  provider:package          (uses latest version)
  package@version           (uses registry provider for curated short names)
  package                   (uses registry provider + latest; for curated short names e.g. ripgrep, uv)

Bare names (no provider:) default to the registry provider (curated github tools).
For mise-managed tools (e.g. go, node) or direct github use 'mise:' or 'github:'.

Examples:
  workspaced tool with github:denoland/deno@1.40.0 -- deno run app.ts
  workspaced tool with ripgrep -- rg pattern
  workspaced tool with uv -- uv --version
  workspaced tool with mise:go@1.21.0 -- go version`,
				Args: cobra.MinimumNArgs(2), // Need at least: tool-spec and command
				RunE: func(cmd *cobra.Command, args []string) error {
					spec := args[0]
					command := args[1]
					commandArgs := args[2:]

					var theCmd *exec.Cmd

					g := taskgroup.MustFromContext(cmd.Context())
					g.Go("tool:with:"+spec, taskgroup.Control, func(ctx context.Context, s *taskgroup.Status) error {
						s.Update("ensuring " + spec)
						s.Progress(0, 1)

						// Perform the tool resolution/install using the task's context.
						// This context is tied to the group and supports cancellation
						// (e.g. ^C during download will abort the fetch/install tasks).
						m, err := tool.NewManager()
						if err != nil {
							return err
						}
						binPath, err := m.EnsureInstalled(ctx, spec, command)
						if err != nil {
							return fmt.Errorf("failed to ensure tool installed: %w", err)
						}

						// Create the final *exec.Cmd using a context *detached* from the
						// task group. taskgroup.Run(g) / Wait() will call cancel() on the
						// group context (to signal renderers done), even on success.
						// If the exec.Cmd was created with CommandContext on a ctx that
						// then gets canceled, the child process gets killed immediately
						// (or fails to start cleanly). The "with" use case needs the
						// launched tool to outlive the "ensuring" + TUI teardown phase.
						execCtx := context.WithoutCancel(ctx)
						c, err := execdriver.Run(execCtx, binPath, commandArgs...)
						if err != nil {
							return err
						}
						theCmd = c
						s.Progress(1, 1)
						return nil
					})

					if err := taskgroup.Run(g); err != nil {
						return err
					}

					if theCmd == nil {
						return nil // nothing to run (shouldn't happen)
					}

					theCmd.Stdin = os.Stdin
					theCmd.Stdout = os.Stdout
					theCmd.Stderr = os.Stderr
					return theCmd.Run()
				},
			}
		})
}
