package home

import (
	"workspaced/internal/cmdregistry"

	"github.com/spf13/cobra"
)

var Registry cmdregistry.CommandRegistry

func GetCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "home",
		Short: "Dotfiles and system state management",
	}
	Registry.FillCommands(cmd)
	return cmd
}
