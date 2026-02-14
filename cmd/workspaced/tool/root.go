package tool

import (
	"github.com/spf13/cobra"
	_ "workspaced/pkg/tool/prelude"
)

func NewCommand() *cobra.Command {
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
