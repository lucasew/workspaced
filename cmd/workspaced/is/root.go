package is

import (
	"workspaced/pkg/cmdregistry"

	"github.com/spf13/cobra"
)

var Registry cmdregistry.CommandRegistry

func GetCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "is",
		Short: "Environment detection commands",
	}
	return Registry.FillCommands(cmd)
}
