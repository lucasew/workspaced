package audio

import (
	"workspaced/internal/cmdregistry"

	"github.com/spf13/cobra"
)

var Registry cmdregistry.CommandRegistry

func GetCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "audio",
		Short: "Control audio volume",
	}
	return Registry.FillCommands(cmd)
}
