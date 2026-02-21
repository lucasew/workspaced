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
			Use:   "add <name> [source]",
			Short: "Add or update a module source in workspaced.mod.toml",
			Long: `Examples:
  workspaced mod add base16-vim remote:base16/vim
  workspaced mod add my-module`,
			Args: cobra.RangeArgs(1, 2),
			RunE: func(cmd *cobra.Command, args []string) error {
				ws, err := modfile.DetectWorkspace(context.Background(), "")
				if err != nil {
					return err
				}

				name := strings.TrimSpace(args[0])
				if name == "" {
					return fmt.Errorf("module name cannot be empty")
				}

				source := "local:" + name
				if len(args) > 1 {
					source = strings.TrimSpace(args[1])
				}
				if !strings.Contains(source, ":") {
					return fmt.Errorf("invalid source %q (expected provider-or-alias:path)", source)
				}
				if strings.HasPrefix(source, "core:") {
					return fmt.Errorf("core modules are built-in; do not add them to workspaced.mod.toml")
				}

				if err := ws.EnsureFiles(); err != nil {
					return err
				}
				mod, err := modfile.LoadModFile(ws.ModPath())
				if err != nil {
					return err
				}
				mod.Modules[name] = source
				if err := modfile.WriteModFile(ws.ModPath(), mod); err != nil {
					return err
				}

				cmd.Printf("updated %s: %s = %s\n", ws.ModPath(), name, source)
				return nil
			},
		})
	})
}
