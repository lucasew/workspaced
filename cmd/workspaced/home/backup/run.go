package backup

import (
	"context"

	"github.com/lucasew/workspaced/internal/backup"
	"github.com/lucasew/workspaced/pkg/taskgroup"
)

type Run struct{}

func (Run) Description() string { return "Run full backup" }

func (*Run) Run(ctx context.Context) error {
	g := taskgroup.MustFromContext(ctx)
	g.Go("backup:run", taskgroup.Control, func(ctx context.Context, s *taskgroup.Status) error {
		s.Update("running backup")
		return backup.RunFullBackup(ctx)
	})
	return nil
}
