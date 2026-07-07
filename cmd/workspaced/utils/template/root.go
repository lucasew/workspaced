package template

import (
	"workspaced/internal/cmdregistry"

	"github.com/spf13/cobra"
)

var Registry cmdregistry.CommandRegistry

func GetCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "template",
		Short: "Template management commands",
	}
	Registry.FillCommands(cmd)
	return cmd
}
