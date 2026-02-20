package home

import (
	"workspaced/cmd/workspaced/home/apply"
	"workspaced/cmd/workspaced/home/config"
	"workspaced/cmd/workspaced/home/plan"
	"workspaced/cmd/workspaced/home/sync"

	"github.com/spf13/cobra"
)

func GetCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "home",
		Short: "Dotfiles and system state management",
	}

	cmd.AddCommand(apply.GetCommand())
	cmd.AddCommand(plan.GetCommand())
	cmd.AddCommand(sync.GetCommand())
	cmd.AddCommand(config.GetCommand())

	return cmd
}
