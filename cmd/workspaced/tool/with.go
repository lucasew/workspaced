package tool

import (
	"os"
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
					c, err := tool.EnsureAndRun(cmd.Context(), spec, command, commandArgs...)
					if err != nil {
						return err
					}
					c.Stdin = os.Stdin
					c.Stdout = os.Stdout
					c.Stderr = os.Stderr
					return c.Run()
				},
			}
		})
}
