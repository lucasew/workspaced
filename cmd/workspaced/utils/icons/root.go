package icons

import (
	"workspaced/pkg/cmdregistry"

	"github.com/spf13/cobra"
)

var Registry cmdregistry.CommandRegistry

func GetCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "icons",
		Short: "Icon theme generation utilities",
	}
	Registry.FillCommands(cmd)
	return cmd
}
