package template

import (
	"workspaced/pkg/registry"

	"github.com/spf13/cobra"
)

var Registry registry.CommandRegistry

func GetCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "template",
		Short: "Template management commands",
	}
	Registry.FillCommands(cmd)
	return cmd
}
