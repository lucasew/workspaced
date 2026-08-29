package backup

import (
	"context"

	"github.com/lucasew/workspaced/internal/backup"
	"github.com/lucasew/workspaced/pkg/taskgroup"

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
					// No Unit(): RunFullBackup uses Map with its own aggregate bar.
					return backup.RunFullBackup(ctx)
				})
				return nil
			},
		})
	})
}
