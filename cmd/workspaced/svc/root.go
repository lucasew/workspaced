package svc

import (
	"github.com/lucasew/workspaced/internal/cmdregistry"

	"github.com/spf13/cobra"
)

var Registry cmdregistry.CommandRegistry

func GetCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "svc",
		Short: "Background services",
	}

	return Registry.FillCommands(cmd)
}
