package experiments

import (
	"encoding/json"
	"fmt"
	"os"

	"workspaced/pkg/configcue"

	"github.com/spf13/cobra"
)

func init() {
	Registry.FromGetter(GetCueCommand)
}

func GetCueCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "cue",
		Short: "Inspect experimental layered CUE configuration",
	}
	cmd.AddCommand(&cobra.Command{
		Use:   "layers",
		Short: "List discovered workspaced.cue layers",
		RunE: func(cmd *cobra.Command, args []string) error {
			cwd, err := os.Getwd()
			if err != nil {
				return err
			}
			res, err := configcue.DiscoverLayers(configcue.DiscoverOptions{Cwd: cwd})
			if err != nil {
				return err
			}
			enc := json.NewEncoder(cmd.OutOrStdout())
			enc.SetIndent("", "  ")
			return enc.Encode(res)
		},
	})
	cmd.AddCommand(&cobra.Command{
		Use:   "export",
		Short: "Export the unified experimental CUE config as JSON",
		RunE: func(cmd *cobra.Command, args []string) error {
			cwd, err := os.Getwd()
			if err != nil {
				return err
			}
			data, err := configcue.ExportJSON(configcue.DiscoverOptions{Cwd: cwd})
			if err != nil {
				return err
			}
			var pretty any
			if err := json.Unmarshal(data, &pretty); err != nil {
				return fmt.Errorf("decode generated json: %w", err)
			}
			enc := json.NewEncoder(cmd.OutOrStdout())
			enc.SetIndent("", "  ")
			return enc.Encode(pretty)
		},
	})
	return cmd
}
