package tool

import (
	"github.com/spf13/cobra"
	"workspaced/pkg/tool"
)

func newInstallCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "install <provider:package@version>",
		Short: "Install a tool",
		Example: `  workspaced tool install github:denoland/deno@1.40.0
  workspaced tool install github:denoland/deno@latest`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			manager, err := tool.NewManager()
			if err != nil {
				return err
			}
			return manager.Install(cmd.Context(), args[0])
		},
	}
}
