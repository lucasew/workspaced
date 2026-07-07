package brightness

import (
	"workspaced/internal/cmdregistry"

	"github.com/spf13/cobra"
)

var Registry cmdregistry.CommandRegistry

func GetCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "brightness",
		Short: "Control screen brightness",
	}
	return Registry.FillCommands(cmd)
}
