package config

import (
	"fmt"
	"os"
	"text/tabwriter"

	"workspaced/pkg/configcue"

	"github.com/spf13/cobra"
)

func init() {
	Registry.FromGetter(GetLayersCommand)
}

func GetLayersCommand() *cobra.Command {
	var format string

	cmd := &cobra.Command{
		Use:   "layers",
		Short: "List discovered workspaced.cue layers",
		RunE: func(c *cobra.Command, args []string) error {
			cwd, _ := os.Getwd()
			result, err := configcue.Evaluate(configcue.DiscoverOptions{
				Cwd:      cwd,
				HomeMode: true,
			})
			if err != nil {
				return fmt.Errorf("failed to discover config layers: %w", err)
			}

			if format == "table" {
				w := tabwriter.NewWriter(c.OutOrStdout(), 0, 0, 2, ' ', 0)
				if _, err := fmt.Fprintln(w, "NAME\tPATH"); err != nil {
					return err
				}
				for _, layer := range result.Layers {
					if _, err := fmt.Fprintf(w, "%s\t%s\n", layer.Name, layer.Path); err != nil {
						return err
					}
				}
				return w.Flush()
			}
			if format != "" && format != "paths" {
				return fmt.Errorf("unknown format: %s (supported: paths, table)", format)
			}
			for _, layer := range result.Layers {
				if _, err := fmt.Fprintln(c.OutOrStdout(), layer.Path); err != nil {
					return err
				}
			}
			return nil
		},
	}

	cmd.Flags().StringVarP(&format, "format", "f", "paths", "Output format (paths, table)")
	return cmd
}
