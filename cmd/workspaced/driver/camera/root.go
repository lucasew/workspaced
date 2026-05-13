package camera

import (
	"workspaced/pkg/registry"

	"github.com/spf13/cobra"
)

var Registry registry.CommandRegistry

func GetCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "camera",
		Short: "Camera capture management",
	}
	return Registry.FillCommands(cmd)
}
