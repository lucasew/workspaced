package camera

import (
	"github.com/spf13/cobra"
	"workspaced/pkg/registry"
)

var Registry registry.CommandRegistry

func GetCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "camera",
		Short: "Camera management",
	}
	return Registry.FillCommands(cmd)
}
