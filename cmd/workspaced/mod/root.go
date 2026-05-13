package mod

import (
	_ "workspaced/pkg/modfile/sourceprovider/prelude"
	"workspaced/pkg/registry"

	"github.com/spf13/cobra"
)

var Registry registry.CommandRegistry

func GetCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "mod",
		Short: "Manage module sources and lockfile",
	}
	Registry.FillCommands(cmd)
	return cmd
}
