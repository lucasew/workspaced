package config

import (
	"os"

	"workspaced/pkg/configcue"

	"github.com/spf13/cobra"
)

func init() {
	Registry.FromGetter(GetDefCommand)
}

func GetDefCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "def",
		Short: "Show merged config definitions/types (cue def-like)",
		Args:  cobra.NoArgs,
		RunE: func(c *cobra.Command, args []string) error {
			cwd, _ := os.Getwd()
			out, err := configcue.ExportDef(configcue.DiscoverOptions{
				Cwd:      cwd,
				HomeMode: true,
			})
			if err != nil {
				return err
			}
			_, err = c.OutOrStdout().Write(out)
			return err
		},
	}
}
