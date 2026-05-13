package svc

import (
	"workspaced/pkg/registry"

	"github.com/spf13/cobra"
)

var Registry registry.CommandRegistry

func GetCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "svc",
		Short: "Background services",
	}

	return Registry.FillCommands(cmd)
}
