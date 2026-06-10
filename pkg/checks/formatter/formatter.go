package formatter

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sync"

	"workspaced/pkg/checks"
	"workspaced/pkg/logging"
	"workspaced/pkg/taskgroup"
)

// Formatter extends the base Check for code formatting tools.
type Formatter interface {
	checks.Check

	// Format applies formatting changes to files in the directory.
	Format(ctx context.Context, dir string) error
}

// Register adds a formatter to the global checks registry.
func Register(f Formatter) {
	checks.Register[Formatter](f)
}

// RunAll executes all applicable formatters in parallel.
func RunAll(ctx context.Context, dir string) error {
	formatters := checks.List[Formatter]()
	slog.Info("Running formatters", "count", len(formatters), "dir", dir)

	// Filter to applicable formatters first.
	var applicable []Formatter
	for _, f := range formatters {
		err := f.Detect(ctx, dir)
		if errors.Is(err, checks.ErrNotApplicable) {
			continue
		}
		if err != nil {
			slog.Warn("formatter detection failed", "name", f.Name(), "error", err)
			continue
		}
		applicable = append(applicable, f)
	}

	if len(applicable) == 0 {
		return nil
	}

	var g *taskgroup.Group
	if parent := taskgroup.FromContext(ctx); parent != nil {
		g, ctx = parent.SubGroup(ctx)
	} else {
		g, ctx = taskgroup.New(ctx, taskgroup.DefaultLimits())
	}

	var mu sync.Mutex
	var errs []error

	for _, f := range applicable {
		fmtr := f
		g.Go(fmt.Sprintf("fmt:%s", fmtr.Name()), taskgroup.CPU, func(ctx context.Context, s *taskgroup.Status) error {
			s.Update(fmt.Sprintf("running %s", fmtr.Name()))
			slog.Info("Running formatter", "name", fmtr.Name())
			if err := fmtr.Format(ctx, dir); err != nil {
				logging.ReportError(ctx, err, slog.String("name", fmtr.Name()), slog.String("context", "formatter failed"))
				mu.Lock()
				errs = append(errs, fmt.Errorf("%s: %w", fmtr.Name(), err))
				mu.Unlock()
			}
			return nil
		})
	}

	if err := g.Wait(); err != nil {
		return err
	}

	if len(errs) > 0 {
		return fmt.Errorf("formatting failed for %d tools: %v", len(errs), errs)
	}
	return nil
}
