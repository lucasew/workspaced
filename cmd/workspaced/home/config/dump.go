package config

import (
	"encoding/json"
	"fmt"

	"workspaced/pkg/config"

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
			cfg, err := config.LoadHome()
			if err != nil {
				return fmt.Errorf("failed to load config: %w", err)
			}

			enc := json.NewEncoder(c.OutOrStdout())
			enc.SetIndent("", "  ")
			return enc.Encode(cfg.Raw())
		},
	}
}
