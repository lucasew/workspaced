package tool

import (
	"workspaced/internal/cmdregistry"
	_ "workspaced/internal/tool/prelude"

	"github.com/spf13/cobra"
)

var Registry cmdregistry.CommandRegistry

func GetCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "tool",
		Short: "Manage development tools",
	}
	Registry.FillCommands(cmd)
	return cmd
}
