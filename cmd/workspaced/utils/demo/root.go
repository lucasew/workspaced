package demo

import (
	"workspaced/internal/cmdregistry"

	"github.com/spf13/cobra"
)

var Registry cmdregistry.CommandRegistry

func GetCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "demo",
		Short: "Demo commands",
	}
	return Registry.FillCommands(cmd)
}
