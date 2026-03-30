package config

import (
	"os"

	"workspaced/pkg/configcue"

	"github.com/spf13/cobra"
)

func init() {
	Registry.FromGetter(GetEvalCommand)
}

func GetEvalCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "eval",
		Short: "Evaluate merged config (cue eval-like)",
		Long: `Evaluate the merged configuration, similarly to cue eval.

Prints the full merged config without filtering.`,
		Args: cobra.NoArgs,
		RunE: func(c *cobra.Command, args []string) error {
			cwd, _ := os.Getwd()
			out, err := configcue.ExportCUE(configcue.DiscoverOptions{
				Cwd: cwd,
			})
			if err != nil {
				return err
			}
			if _, err := c.OutOrStdout().Write(out); err != nil {
				return err
			}
			return nil
		},
	}
}
