package tool

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"workspaced/internal/tool"
	execdriver "workspaced/pkg/driver/exec"
	"workspaced/pkg/taskgroup"

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
						// Resolution/install runs under this task's ctx so ^C cancels fetches.
						// No Unit(): the ensure Map owns the aggregate bar.
						m, err := tool.NewManager()
						if err != nil {
							return err
						}

						type specItem struct {
							index int
							spec  string
						}
						type binOutcome struct {
							index   int
							binPath string
							miss    bool
						}
						items := make([]specItem, len(toolSpecs))
						for i, spec := range toolSpecs {
							items[i] = specItem{index: i, spec: spec}
						}

						// Map installs each spec (with command as binary hint). Soft-miss when
						// the tool tree has no such binary; hard errors fail the map.
						// Reduce picks the rightmost hit so "with nodejs uv -- node" still
						// resolves node from nodejs even if uv is listed last.
						outcomes, err := taskgroup.Map[specItem, binOutcome]{
							Name:     "tool-with:ensure",
							Items:    items,
							PoolKind: taskgroup.Control,
							TaskName: func(_ int, it specItem) string { return "ensure:" + it.spec },
							Fn: func(ctx context.Context, st *taskgroup.Status, it specItem) (binOutcome, error) {
								st.Update(it.spec)
								bp, err := m.EnsureInstalled(ctx, it.spec, command)
								if err == nil {
									return binOutcome{index: it.index, binPath: bp}, nil
								}
								if isBinaryNotFound(err) {
									return binOutcome{index: it.index, miss: true}, nil
								}
								return binOutcome{}, fmt.Errorf("ensure tool %s: %w", it.spec, err)
							},
						}.Run(ctx)
						if err != nil {
							return err
						}

						binPath := ""
						for i := len(outcomes) - 1; i >= 0; i-- {
							if !outcomes[i].miss && outcomes[i].binPath != "" {
								binPath = outcomes[i].binPath
								break
							}
						}
						if binPath == "" {
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

func isBinaryNotFound(err error) bool {
	return errors.Is(err, tool.ErrBinaryNotFound) || strings.Contains(err.Error(), "binary not found")
}
