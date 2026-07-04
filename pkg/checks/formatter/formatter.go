package formatter

import (
	"context"
	"errors"
	"fmt"
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
	logger := logging.GetLogger(ctx)
	formatters := checks.List[Formatter]()
	logger.Info("running formatters", "count", len(formatters), "dir", dir)

	// Filter to applicable formatters first.
	var applicable []Formatter
	for _, f := range formatters {
		err := f.Detect(ctx, dir)
		if errors.Is(err, checks.ErrNotApplicable) {
			continue
		}
		if err != nil {
			logger.Warn("formatter detection failed", "name", f.Name(), "error", err)
			continue
		}
		applicable = append(applicable, f)
	}

	if len(applicable) == 0 {
		return nil
	}

	var mu sync.Mutex
	var errs []error
	_, err := taskgroup.Map(ctx, "format",
		func(Formatter) taskgroup.PoolKind { return taskgroup.CPU },
		applicable,
		func(_ int, f Formatter) string { return "fmt:" + f.Name() },
		func(ctx context.Context, s *taskgroup.Status, fmtr Formatter) (struct{}, error) {
			l := logging.GetLogger(ctx)
			s.Update("running " + fmtr.Name())
			l.Info("running formatter", "name", fmtr.Name())
			if err := fmtr.Format(ctx, dir); err != nil {
				logging.ReportError(ctx, err, "name", fmtr.Name(), "context", "formatter failed")
				mu.Lock()
				errs = append(errs, fmt.Errorf("%s: %w", fmtr.Name(), err))
				mu.Unlock()
			}
			return struct{}{}, nil
		})
	if err != nil {
		return err
	}
	if len(errs) > 0 {
		return fmt.Errorf("formatting failed for %d tools: %w", len(errs), errors.Join(errs...))
	}
	return nil
}
