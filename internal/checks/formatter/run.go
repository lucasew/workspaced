package formatter

import (
	"context"
	"errors"
	"fmt"

	"github.com/lucasew/workspaced/internal/checks"
	"github.com/lucasew/workspaced/pkg/logging"
	"github.com/lucasew/workspaced/pkg/taskgroup"
)

// RunAll loads CUE formatter tools and runs applicable ones serially.
func RunAll(ctx context.Context, dir string) error {
	logger := logging.GetLogger(ctx)
	tools, err := checks.LoadToolsForDir(ctx, dir, "formatter")
	if err != nil {
		return err
	}
	logger.Info("running formatters", "count", len(tools), "dir", dir)

	type item struct {
		tool   checks.Tool
		detect checks.DetectResult
	}
	var applicable []item
	for _, t := range tools {
		if !t.Enable {
			continue
		}
		det, err := checks.EvaluateDetect(dir, t.Detect)
		if err != nil {
			logging.ReportError(ctx, err, "tool", t.Name, "context", "formatter detect")
			continue
		}
		if !det.Applicable {
			continue
		}
		applicable = append(applicable, item{tool: t, detect: det})
	}
	if len(applicable) == 0 {
		return nil
	}

	// Soft-collect per-tool failures in U so one bad formatter does not cancel
	// siblings (Map shares one SubGroup; a hard Fn error would). Hard error is
	// only for Map/taskgroup failure. nil *toolFailure means that tool succeeded.
	// Control: ResolveCmd may EnsureInstalled (httpclient Internet).
	failures, err := taskgroup.Map[item, *toolFailure]{
		Name:     "format",
		Items:    applicable,
		PoolKind: taskgroup.Control,
		Serial:   true,
		TaskName: func(_ int, it item) string { return "fmt:" + it.tool.Name },
		Fn: func(ctx context.Context, s *taskgroup.Status, it item) (*toolFailure, error) {
			l := logging.GetLogger(ctx)
			s.Update("running " + it.tool.Name)
			l.Info("running formatter", "name", it.tool.Name)
			if err := runOne(ctx, dir, it.tool, it.detect); err != nil {
				logging.ReportError(ctx, err, "name", it.tool.Name, "context", "formatter failed")
				return &toolFailure{name: it.tool.Name, err: err}, nil
			}
			return nil, nil
		},
	}.Run(ctx)
	if err != nil {
		return err
	}
	var errs []error
	for _, f := range failures {
		if f != nil {
			errs = append(errs, f)
		}
	}
	if len(errs) == 0 {
		return nil
	}
	return fmt.Errorf("formatting failed for %d tools: %w", len(errs), errors.Join(errs...))
}

// toolFailure is a Map result (soft fail). Not the task hard-fail channel.
type toolFailure struct {
	name string
	err  error
}

func (f *toolFailure) Error() string {
	return f.name + ": " + f.err.Error()
}

func (f *toolFailure) Unwrap() error { return f.err }

func runOne(ctx context.Context, dir string, t checks.Tool, det checks.DetectResult) error {
	cmd, err := checks.BuildCmd(ctx, dir, t, det)
	if err != nil {
		return err
	}
	return checks.RunAttached(cmd, dir)
}
