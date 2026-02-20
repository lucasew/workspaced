package tool

import (
	_ "workspaced/pkg/tool/prelude"

	"github.com/spf13/cobra"
)

func GetCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "tool",
		Short: "Manage development tools",
	}
	cmd.AddCommand(
		newInstallCommand(),
		newListCommand(),
		newWithCommand(),
	)
	return cmd
}
