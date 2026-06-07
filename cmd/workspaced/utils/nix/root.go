package nix

import (
	"workspaced/pkg/cmdregistry"

	"github.com/spf13/cobra"
)

var Registry cmdregistry.CommandRegistry

func GetCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "nix",
		Short: "Nix operations",
	}
	return Registry.FillCommands(cmd)
}
