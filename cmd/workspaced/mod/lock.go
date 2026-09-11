package mod

import (
	"context"

	"github.com/lucasew/workspaced/internal/modfile"
	"github.com/lucasew/workspaced/pkg/logging"
	"github.com/lucasew/workspaced/pkg/taskgroup"
)

type Lock struct{}

func (Lock) Description() string {
	return "Refresh workspaced.lock.json for enabled modules"
}

func (c *Lock) Run(ctx context.Context) error {
	return runModLock(ctx)
}

type Tidy struct{}

func (Tidy) Description() string {
	return "Alias for `mod lock`"
}

func (c *Tidy) Run(ctx context.Context) error {
	return runModLock(ctx)
}

func runModLock(ctx context.Context) error {
	g := taskgroup.MustFromContext(ctx)
	g.Go("mod:lock", taskgroup.Control, func(ctx context.Context, s *taskgroup.Status) error {
		s.Update("refreshing lockfile")
		logger := logging.GetLogger(ctx)
		ws, err := modfile.DetectWorkspace(ctx, "")
		if err != nil {
			return err
		}
		result, err := modfile.GenerateLock(ctx, ws)
		if err != nil {
			return err
		}
		if result.Changed {
			logger.Info("wrote lockfile", "path", ws.SumPath(), "sources", result.Sources)
		} else {
			logger.Info("lockfile up to date", "path", ws.SumPath(), "sources", result.Sources)
		}
		return nil
	})
	return nil
}
