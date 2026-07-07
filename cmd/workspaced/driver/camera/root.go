package camera

import (
	"workspaced/internal/cmdregistry"

	"github.com/spf13/cobra"
)

var Registry cmdregistry.CommandRegistry

func GetCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "camera",
		Short: "Camera capture management",
	}
	return Registry.FillCommands(cmd)
}
