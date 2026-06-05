package backup

import (
	"workspaced/pkg/cmdregistry"

	"github.com/spf13/cobra"
)

var Registry cmdregistry.CommandRegistry

func GetCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "backup",
		Short: "Data backup and synchronization",
	}
	return Registry.FillCommands(cmd)
}
