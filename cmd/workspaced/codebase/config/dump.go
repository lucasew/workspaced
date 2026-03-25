package config

import (
	"encoding/json"
	"fmt"
	"os"

	"workspaced/pkg/configcue"

	"github.com/spf13/cobra"
)

func init() {
	Registry.FromGetter(GetDumpCommand)
}

func GetDumpCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "dump",
		Short: "Dump the full merged configuration as JSON",
		Long: `Dump the complete merged configuration from all sources:
- Hardcoded defaults
- layered workspaced.cue files

Outputs the result as JSON format.`,
		RunE: func(c *cobra.Command, args []string) error {
			cwd, _ := os.Getwd()
			result, err := configcue.Evaluate(configcue.DiscoverOptions{
				Cwd: cwd,
			})
			if err != nil {
				return fmt.Errorf("failed to load config: %w", err)
			}

			var raw any
			if err := json.Unmarshal(result.JSON, &raw); err != nil {
				return fmt.Errorf("failed to decode evaluated config: %w", err)
			}
			enc := json.NewEncoder(c.OutOrStdout())
			enc.SetIndent("", "  ")
			return enc.Encode(raw)
		},
	}
}
