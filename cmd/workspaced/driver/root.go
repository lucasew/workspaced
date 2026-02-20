package driver

import (
	"workspaced/pkg/registry"

	"github.com/spf13/cobra"
)

var Registry registry.CommandRegistry

func GetCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "driver",
		Short: "Commands to interact with drivers",
	}
	Registry.FillCommands(cmd)
	return cmd
}
