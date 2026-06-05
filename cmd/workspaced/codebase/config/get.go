package config

import (
	"encoding/json"
	"fmt"

	"workspaced/pkg/configcue"

	"github.com/spf13/cobra"
)

func init() {
	Registry.FromGetter(GetGetCommand)
}

func GetGetCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "get <key>",
		Short: "Get a configuration value (outputs JSON)",
		Long: `Get a configuration value using dot notation.

Examples:
  workspaced codebase config get workspaces.www
  workspaced codebase config get desktop.wallpaper.dir
  workspaced codebase config get desktop.wallpaper

Outputs the value as JSON for easy parsing.`,
		Args: cobra.ExactArgs(1),
		RunE: func(c *cobra.Command, args []string) error {
			key := args[0]
			cfg, err := configcue.Load()
			if err != nil {
				return fmt.Errorf("failed to load config: %w", err)
			}

			result, err := getConfigValue(cfg, key)
			if err != nil {
				return err
			}

			jsonBytes, err := json.MarshalIndent(result, "", "  ")
			if err != nil {
				return fmt.Errorf("failed to encode JSON: %w", err)
			}

			c.Println(string(jsonBytes))
			return nil
		},
	}
}

func getConfigValue(cfg *configcue.Config, key string) (any, error) {
	if key == "" {
		return cfg.Raw(), nil
	}

	return cfg.Lookup(key)
}
