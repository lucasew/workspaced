package mod

import (
	"context"
	"fmt"
	"strings"
	"workspaced/pkg/modfile"

	"github.com/spf13/cobra"
)

func init() {
	Registry.Register(func(c *cobra.Command) {
		c.AddCommand(&cobra.Command{
			Use:   "add <source-alias> <provider>",
			Short: "Add or update a source alias in workspaced.mod.toml",
			Long: `Examples:
  workspaced mod add papirus core
  workspaced mod add mymods local`,
			Args: cobra.ExactArgs(2),
			RunE: func(cmd *cobra.Command, args []string) error {
				ws, err := modfile.DetectWorkspace(context.Background(), "")
				if err != nil {
					return err
				}

				alias := strings.TrimSpace(args[0])
				if alias == "" {
					return fmt.Errorf("source alias cannot be empty")
				}

				provider := strings.TrimSpace(args[1])
				if provider == "" {
					return fmt.Errorf("provider cannot be empty")
				}

				if err := ws.EnsureFiles(); err != nil {
					return err
				}
				mod, err := modfile.LoadModFile(ws.ModPath())
				if err != nil {
					return err
				}
				src := mod.Sources[alias]
				src.Provider = provider
				mod.Sources[alias] = src
				if err := modfile.WriteModFile(ws.ModPath(), mod); err != nil {
					return err
				}
				result, err := modfile.GenerateLock(context.Background(), ws)
				if err != nil {
					return err
				}

				cmd.Printf("updated %s: [sources.%s].provider = %s\n", ws.ModPath(), alias, provider)
				cmd.Printf("wrote %s (%d sources, %d modules)\n", ws.SumPath(), result.Sources, result.Modules)
				return nil
			},
		})
	})
}
