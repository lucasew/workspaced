package screenshot

import (
	"workspaced/pkg/cmdregistry"

	"github.com/spf13/cobra"
)

var Registry cmdregistry.CommandRegistry

func GetCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "screenshot",
		Short: "Screen capture management",
	}
	return Registry.FillCommands(cmd)
}
