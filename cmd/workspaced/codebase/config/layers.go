package config

import (
	"encoding/json"
	"fmt"
	"os"

	"workspaced/pkg/configcue"

	"github.com/spf13/cobra"
)

func init() {
	Registry.FromGetter(GetLayersCommand)
}

func GetLayersCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "layers",
		Short: "List discovered workspaced.cue layers",
		RunE: func(c *cobra.Command, args []string) error {
			cwd, _ := os.Getwd()
			result, err := configcue.Evaluate(configcue.DiscoverOptions{
				Cwd: cwd,
			})
			if err != nil {
				return fmt.Errorf("failed to discover config layers: %w", err)
			}

			enc := json.NewEncoder(c.OutOrStdout())
			enc.SetIndent("", "  ")
			return enc.Encode(map[string]any{"layers": result.Layers})
		},
	}
}
