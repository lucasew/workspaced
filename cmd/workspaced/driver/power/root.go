package power

import (
	"workspaced/internal/cmdregistry"

	"github.com/spf13/cobra"
)

var Registry cmdregistry.CommandRegistry

func GetCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "power",
		Short: "Power management commands",
	}
	return Registry.FillCommands(cmd)
}
