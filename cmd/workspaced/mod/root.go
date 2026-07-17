package mod

import (
	"workspaced/pkg/cmdregistry"
	_ "workspaced/pkg/modfile/sourceprovider/prelude"

	"github.com/spf13/cobra"
)

var Registry cmdregistry.CommandRegistry

func GetCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "mod",
		Short: "Manage module sources and lockfile",
	}
	Registry.FillCommands(cmd)
	return cmd
}
