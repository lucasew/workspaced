package experiments

import (
	"workspaced/pkg/registry"

	"github.com/spf13/cobra"
)

var Registry registry.CommandRegistry

func GetCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "experiments",
		Short: "Experimental features and prototypes",
	}
	Registry.FillCommands(cmd)
	return cmd
}
