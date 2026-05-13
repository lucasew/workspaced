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
  package@version           (uses registry provider - not yet implemented)
  package                   (uses registry provider and latest version - not yet implemented)

Note: Currently you must specify the provider explicitly (e.g., github:package@version)

Examples:
  workspaced tool with github:denoland/deno@1.40.0 -- deno run app.ts
  workspaced tool with denoland/deno -- deno --version
  workspaced tool with deno@1.40.0 -- deno run app.ts
  workspaced tool with ripgrep -- rg pattern`,
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
