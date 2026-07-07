package wallpaper

import (
	"workspaced/internal/cmdregistry"

	"github.com/spf13/cobra"
)

var Registry cmdregistry.CommandRegistry

func GetCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "wallpaper",
		Short: "Wallpaper management",
	}
	return Registry.FillCommands(cmd)
}
