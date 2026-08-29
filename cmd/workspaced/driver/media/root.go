package media

import (
	"github.com/lucasew/workspaced/internal/cmdregistry"

	"github.com/spf13/cobra"
)

var Registry cmdregistry.CommandRegistry

func GetCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "media",
		Short: "Control media playback",
	}
	return Registry.FillCommands(cmd)
}
