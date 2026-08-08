package palette

import (
	"workspaced/pkg/cmdregistry"

	"github.com/spf13/cobra"
)

var Registry cmdregistry.CommandRegistry

func GetCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "palette",
		Short: "Color palette generation and management",
		Long:  "Generate base16/base24 color palettes from images using pluggable extraction drivers",
	}
	return Registry.FillCommands(cmd)
}
