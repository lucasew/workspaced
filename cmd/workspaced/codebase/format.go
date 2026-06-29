package codebase

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"workspaced/pkg/checks/formatter"
	_ "workspaced/pkg/checks/prelude"
	"workspaced/pkg/git"
	"workspaced/pkg/taskgroup"

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

				ctx := cmd.Context()
				g := taskgroup.MustFromContext(ctx)
				g.Go("codebase:format", taskgroup.Control, func(ctx context.Context, s *taskgroup.Status) error {
					s.Update("running formatters")
					s.Progress(0, 1)
					defer s.Progress(1, 1)
					return formatter.RunAll(ctx, root)
				})
				return nil
			},
		})
	})
}
