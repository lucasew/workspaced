package mod

import (
	"github.com/lucasew/workspaced/internal/cmdregistry"
	_ "github.com/lucasew/workspaced/internal/modfile/sourceprovider/prelude"

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
