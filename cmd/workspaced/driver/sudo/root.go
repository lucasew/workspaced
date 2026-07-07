package sudo

import (
	"workspaced/internal/cmdregistry"

	"github.com/spf13/cobra"
)

var Registry cmdregistry.CommandRegistry

func GetCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "sudo",
		Short: "Manage pending privileged commands",
	}
	return Registry.FillCommands(cmd)
}
