package home

import (
	"workspaced/pkg/registry"

	"github.com/spf13/cobra"
)

var Registry registry.CommandRegistry

func GetCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "home",
		Short: "Dotfiles and system state management",
	}
	Registry.FillCommands(cmd)
	return cmd
}
