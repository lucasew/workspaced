package screen

import (
	"workspaced/pkg/cmdregistry"

	"github.com/spf13/cobra"
)

var Registry cmdregistry.CommandRegistry

func GetCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "screen",
		Short: "Screen and power management",
	}
	return Registry.FillCommands(cmd)
}
