package backup

import (
	"context"

	"workspaced/pkg/backup"
	"workspaced/pkg/taskgroup"

	"github.com/spf13/cobra"
)

func init() {
	Registry.Register(func(parent *cobra.Command) {
		parent.AddCommand(&cobra.Command{
			Use:   "run",
			Short: "Run full backup",
			RunE: func(c *cobra.Command, args []string) error {
				g := taskgroup.MustFromContext(c.Context())
				g.Go("backup:run", taskgroup.Control, func(ctx context.Context, s *taskgroup.Status) error {
					s.Update("running backup")
					s.Progress(0, 1)
					err := backup.RunFullBackup(ctx)
					s.Progress(1, 1)
					return err
				})
				return nil
			},
		})
	})
}
