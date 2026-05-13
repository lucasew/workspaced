package icons

import (
	"workspaced/pkg/registry"

	"github.com/spf13/cobra"
)

var Registry registry.CommandRegistry

func GetCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "icons",
		Short: "Icon theme generation utilities",
	}
	Registry.FillCommands(cmd)
	return cmd
}
