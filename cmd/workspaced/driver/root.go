package driver

import (
	"workspaced/pkg/cmdregistry"

	"github.com/spf13/cobra"
)

var Registry cmdregistry.CommandRegistry

func GetCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "driver",
		Short: "Commands to interact with drivers",
	}
	Registry.FillCommands(cmd)
	return cmd
}
