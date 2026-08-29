package workspace

import (
	"github.com/lucasew/workspaced/internal/cmdregistry"

	"github.com/spf13/cobra"
)

var Registry cmdregistry.CommandRegistry

func GetCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "workspace",
		Short: "Workspace management commands",
	}
	cmd.PersistentFlags().Bool("move", false, "Move container to workspace")
	return Registry.FillCommands(cmd)
}
