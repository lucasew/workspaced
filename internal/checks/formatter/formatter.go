package formatter

import (
	"context"
	"errors"
	"fmt"

	"workspaced/internal/checks"
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

// RunAll executes all applicable formatters in parallel and joins per-tool errors.
func RunAll(ctx context.Context, dir string) error {
	logger := logging.GetLogger(ctx)
	formatters := checks.List[Formatter]()
	logger.Info("running formatters", "count", len(formatters), "dir", dir)

	applicable := checks.Applicable(ctx, dir, formatters, checks.LogDetectFailures(ctx, "formatter"))
	if len(applicable) == 0 {
		return nil
	}

	perTool, err := taskgroup.Map[Formatter, error]{
		Name:     "format",
		Items:    applicable,
		PoolKind: taskgroup.CPU,
		TaskName: func(_ int, f Formatter) string { return "fmt:" + f.Name() },
		Fn: func(ctx context.Context, s *taskgroup.Status, fmtr Formatter) (error, error) {
			l := logging.GetLogger(ctx)
			s.Update("running " + fmtr.Name())
			l.Info("running formatter", "name", fmtr.Name())
			if err := fmtr.Format(ctx, dir); err != nil {
				logging.ReportError(ctx, err, "name", fmtr.Name(), "context", "formatter failed")
				return fmt.Errorf("%s: %w", fmtr.Name(), err), nil
			}
			return nil, nil
		},
	}.Run(ctx)
	if err != nil {
		return err
	}
	var errs []error
	for _, e := range perTool {
		if e != nil {
			errs = append(errs, e)
		}
	}
	if len(errs) == 0 {
		return nil
	}
	return fmt.Errorf("formatting failed for %d tools: %w", len(errs), errors.Join(errs...))
}
