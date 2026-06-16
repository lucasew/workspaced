package codebase

import (
	"fmt"
	"os"
	"path/filepath"

	"workspaced/pkg/checks/formatter"
	_ "workspaced/pkg/checks/prelude"
	"workspaced/pkg/git"

	"github.com/spf13/cobra"
)

func init() {
	Registry.Register(func(c *cobra.Command) {
		c.AddCommand(&cobra.Command{
			Use:   "format [path]",
			Short: "Format code in the repository (runs at git root)",
			RunE: func(cmd *cobra.Command, args []string) error {
				path, err := os.Getwd()
				if err != nil {
					return err
				}
				if len(args) > 0 {
					path = args[0]
				}

				absPath, err := filepath.Abs(path)
				if err != nil {
					return err
				}

				root, err := git.GetRoot(cmd.Context(), absPath)
				if err != nil {
					return fmt.Errorf("failed to find git root (format must run inside a git repo): %w", err)
				}

				return formatter.RunAll(cmd.Context(), root)
			},
		})
	})
}
