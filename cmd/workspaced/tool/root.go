package tool

import (
	"workspaced/pkg/registry"
	_ "workspaced/pkg/tool/prelude"

	"github.com/spf13/cobra"
)

var Registry registry.CommandRegistry

func GetCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "tool",
		Short: "Manage development tools",
	}
	Registry.FillCommands(cmd)
	return cmd
}
