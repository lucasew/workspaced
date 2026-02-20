package tool

import (
	"workspaced/pkg/tool"

	"github.com/spf13/cobra"
)

func init() {
	Registry.Register(func(c *cobra.Command) {
		c.AddCommand(&cobra.Command{
			Use:   "install <tool-spec>",
			Short: "Install a tool",
			Long: `Install a tool from a provider.

Tool spec format:
  provider:package@version  (full spec)
  provider:package          (uses latest version)
  package@version           (uses registry provider - not yet implemented)
  package                   (uses registry provider and latest version - not yet implemented)

Note: Currently you must specify the provider explicitly (e.g., github:package@version)`,
			Example: `  workspaced tool install github:denoland/deno@1.40.0
  workspaced tool install denoland/deno@latest
  workspaced tool install deno
  workspaced tool install ripgrep@14.0.0`,
			Args: cobra.ExactArgs(1),
			RunE: func(cmd *cobra.Command, args []string) error {
				manager, err := tool.NewManager()
				if err != nil {
					return err
				}
				return manager.Install(cmd.Context(), args[0])
			},
		})
	})

}
