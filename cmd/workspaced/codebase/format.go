package codebase

import (
	"fmt"
	"os"
	"path/filepath"

	"workspaced/pkg/git"
	"workspaced/pkg/provider/formatter"
	_ "workspaced/pkg/provider/prelude"

	"github.com/spf13/cobra"
)

func newFormatCommand() *cobra.Command {
	return &cobra.Command{
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

			// Resolve path to absolute
			absPath, err := filepath.Abs(path)
			if err != nil {
				return err
			}

			// Find git root
			root, err := git.GetRoot(cmd.Context(), absPath)
			if err != nil {
				return fmt.Errorf("failed to find git root (format must run inside a git repo): %w", err)
			}

			// Run formatters at git root
			return formatter.RunAll(cmd.Context(), root)
		},
	}
}
