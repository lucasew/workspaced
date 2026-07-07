package experiments

import (
	"workspaced/internal/cmdregistry"

	"github.com/spf13/cobra"
)

var Registry cmdregistry.CommandRegistry

func GetCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "experiments",
		Short: "Experimental features and prototypes",
	}
	Registry.FillCommands(cmd)
	return cmd
}
