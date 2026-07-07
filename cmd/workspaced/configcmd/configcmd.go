// Package configcmd builds the shared "config" cobra subtree used by both
// "workspaced home config" and "workspaced codebase config".
// The only behavioral fork is HomeMode (which layers are discovered / which
// Load* helper is used) plus the scope name embedded in example text.
package configcmd

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"text/tabwriter"

	"workspaced/internal/configcue"

	"github.com/spf13/cobra"
)

// Options selects home vs codebase config discovery.
type Options struct {
	// HomeMode discovers dotfiles/user/home layers instead of the repo layer.
	HomeMode bool
	// Scope is the CLI path segment shown in examples ("home" or "codebase").
	Scope string
}

func (o Options) discover() configcue.DiscoverOptions {
	cwd, _ := os.Getwd()
	return configcue.DiscoverOptions{Cwd: cwd, HomeMode: o.HomeMode}
}

func (o Options) load(ctx context.Context) (*configcue.Config, error) {
	if o.HomeMode {
		return configcue.LoadHome(ctx)
	}
	return configcue.Load(ctx)
}

// New returns the "config" command with dump/get/eval/def/layers children.
func New(opts Options) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "config",
		Short: "Manage configuration",
	}
	cmd.AddCommand(
		newDumpCommand(opts),
		newGetCommand(opts),
		newEvalCommand(opts),
		newDefCommand(opts),
		newLayersCommand(opts),
	)
	return cmd
}

func newDumpCommand(opts Options) *cobra.Command {
	return &cobra.Command{
		Use:   "dump",
		Short: "Dump the full merged configuration as JSON",
		Long: `Dump the complete merged configuration from all sources:
- Hardcoded defaults
- layered workspaced.cue files

Outputs the result as JSON format.`,
		RunE: func(c *cobra.Command, args []string) error {
			result, err := configcue.Evaluate(c.Context(), opts.discover())
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

func newGetCommand(opts Options) *cobra.Command {
	scope := opts.Scope
	if scope == "" {
		scope = "config"
	}
	return &cobra.Command{
		Use:   "get <key>",
		Short: "Get a configuration value (outputs JSON)",
		Long: fmt.Sprintf(`Get a configuration value using dot notation.

Examples:
  workspaced %s config get workspaces.www
  workspaced %s config get desktop.wallpaper.dir
  workspaced %s config get desktop.wallpaper

Outputs the value as JSON for easy parsing.`, scope, scope, scope),
		Args: cobra.ExactArgs(1),
		RunE: func(c *cobra.Command, args []string) error {
			cfg, err := opts.load(c.Context())
			if err != nil {
				return fmt.Errorf("failed to load config: %w", err)
			}

			result, err := lookupConfigValue(cfg, args[0])
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

func lookupConfigValue(cfg *configcue.Config, key string) (any, error) {
	if key == "" {
		return cfg.Raw(), nil
	}
	return cfg.Lookup(key)
}

func newEvalCommand(opts Options) *cobra.Command {
	return &cobra.Command{
		Use:   "eval",
		Short: "Evaluate merged config (cue eval-like)",
		Long: `Evaluate the merged configuration, similarly to cue eval.

Prints the full merged config without filtering.`,
		Args: cobra.NoArgs,
		RunE: func(c *cobra.Command, args []string) error {
			out, err := configcue.ExportCUE(c.Context(), opts.discover())
			if err != nil {
				return err
			}
			_, err = c.OutOrStdout().Write(out)
			return err
		},
	}
}

func newDefCommand(opts Options) *cobra.Command {
	return &cobra.Command{
		Use:   "def",
		Short: "Show merged config definitions/types (cue def-like)",
		Args:  cobra.NoArgs,
		RunE: func(c *cobra.Command, args []string) error {
			out, err := configcue.ExportDef(c.Context(), opts.discover())
			if err != nil {
				return err
			}
			_, err = c.OutOrStdout().Write(out)
			return err
		},
	}
}

func newLayersCommand(opts Options) *cobra.Command {
	var format string

	cmd := &cobra.Command{
		Use:   "layers",
		Short: "List discovered workspaced.cue layers",
		RunE: func(c *cobra.Command, args []string) error {
			result, err := configcue.Evaluate(c.Context(), opts.discover())
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
