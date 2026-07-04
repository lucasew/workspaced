package tool

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"

	execdriver "workspaced/pkg/driver/exec"
	"workspaced/pkg/taskgroup"
	"workspaced/pkg/tool"

	"github.com/spf13/cobra"
)

func init() {
	Registry.FromGetter(
		func() *cobra.Command {
			return &cobra.Command{
				Use:   "with <tool-spec>... -- <command> [args...]",
				Short: "Execute a command with specific tool version(s)",
				Long: `Execute a command with specific tool version(s).

All <tool-spec> arguments before "--" are ensured (installed if missing).
The <command> is resolved from the first (rightmost) listed tool that provides a binary
of that name, so the backend that owns the command does not have to be the last entry.

If a tool is not installed, it will be installed automatically.

Tool spec format:
  backend:package@version  (full spec)
  backend:package          (uses latest version)
  package@version          (uses registry backend for curated short names)
  package                  (uses registry backend + latest; for curated short names e.g. ripgrep, uv)

Bare names (no backend:) default to the registry backend (curated github tools).
For mise-managed tools (e.g. go, node) or direct github use 'mise:' or 'github:'.

Examples:
  workspaced tool with github:denoland/deno@1.40.0 -- deno run app.ts
  workspaced tool with ripgrep -- rg pattern
  workspaced tool with uv -- uv --version
  workspaced tool with mise:go@1.21.0 -- go version
  workspaced tool with mise:go@1.21.0 mise:node@20 -- node --version
  workspaced tool with nodejs uv -- node --help`,
				Args:               cobra.MinimumNArgs(2), // Need at least: tool-spec and command
				DisableFlagParsing: true,
				RunE: func(cmd *cobra.Command, args []string) error {
					// Parse tools before "--" (supports multiple). If no "--" present,
					// fall back to legacy single-tool mode for compatibility.
					var toolSpecs []string
					var cmdLine []string
					foundDash := false
					for i, a := range args {
						if a == "--" {
							toolSpecs = args[:i]
							cmdLine = args[i+1:]
							foundDash = true
							break
						}
					}
					if !foundDash {
						if len(args) < 2 {
							return fmt.Errorf("at least one tool spec and a command are required (use -- to separate multiple tools)")
						}
						toolSpecs = args[:1]
						cmdLine = args[1:]
					}
					if len(toolSpecs) == 0 || len(cmdLine) == 0 {
						return fmt.Errorf("usage: workspaced tool with <tool-spec>... -- <command> [args...]")
					}

					command := cmdLine[0]
					commandArgs := cmdLine[1:]

					var theCmd *exec.Cmd

					g := taskgroup.MustFromContext(cmd.Context())
					g.Go("tool:with:"+strings.Join(toolSpecs, "+"), taskgroup.Control, func(ctx context.Context, s *taskgroup.Status) error {
						// Perform the tool resolution/install using the task's context.
						// This context is tied to the group and supports cancellation
						// (e.g. ^C during download will abort the fetch/install tasks).
						m, err := tool.NewManager()
						if err != nil {
							return err
						}

						// Ensure side tools (everything but we will search for the command provider below).
						// Using Ensure (not EnsureInstalled) so we don't require the command name to exist
						// in every listed tool.
						for _, specStr := range toolSpecs[:len(toolSpecs)-1] {
							s.Update("ensuring " + specStr)
							if err := m.Ensure(ctx, specStr); err != nil {
								return fmt.Errorf("failed to ensure tool %s: %w", specStr, err)
							}
						}

						// The command name may be provided by any of the listed tools (search from the end
						// so the rightmost one that matches wins). This makes "with nodejs uv -- node"
						// work naturally even if "nodejs" is not the last entry.
						var binPath string
						var found bool
						for i := len(toolSpecs) - 1; i >= 0; i-- {
							spec := toolSpecs[i]
							s.Update("ensuring " + spec)
							bp, err := m.EnsureInstalled(ctx, spec, command)
							if err == nil {
								binPath = bp
								found = true
								break
							}
							if errors.Is(err, tool.ErrBinaryNotFound) || strings.Contains(err.Error(), "binary not found") {
								// This tool doesn't contain the requested command; try the previous one in the list.
								continue
							}
							// Hard failure (resolution, network, etc.) for a candidate -> surface it.
							return fmt.Errorf("failed to ensure tool %s: %w", spec, err)
						}
						if !found {
							return fmt.Errorf("none of the tools (%s) provide a binary named %q", strings.Join(toolSpecs, ", "), command)
						}

						// Detach from the task group so Session.Close (Wait + UI teardown)
						// does not cancel the child via CommandContext.
						execCtx := context.WithoutCancel(ctx)
						c, err := execdriver.Run(execCtx, binPath, commandArgs...)
						if err != nil {
							return err
						}
						theCmd = c
						return nil
					})

					// Run child after session Close (Wait + UI/stderr restore) so it
					// inherits real stdio. PostRun Close runs this via AfterWait.
					taskgroup.MustSessionFrom(cmd.Context()).AfterWait(func() error {
						if theCmd == nil {
							return nil
						}
						theCmd.Stdin = os.Stdin
						theCmd.Stdout = os.Stdout
						theCmd.Stderr = os.Stderr
						return theCmd.Run()
					})
					return nil
				},
			}
		})
}
