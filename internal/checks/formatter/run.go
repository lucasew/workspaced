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

	perTool, err := taskgroup.Map[item, error]{
		Name:     "format",
		Items:    applicable,
		PoolKind: taskgroup.CPU,
		Serial:   true,
		TaskName: func(_ int, it item) string { return "fmt:" + it.tool.Name },
		Fn: func(ctx context.Context, s *taskgroup.Status, it item) (error, error) {
			l := logging.GetLogger(ctx)
			s.Update("running " + it.tool.Name)
			l.Info("running formatter", "name", it.tool.Name)
			if err := runOne(ctx, dir, it.tool, it.detect); err != nil {
				logging.ReportError(ctx, err, "name", it.tool.Name, "context", "formatter failed")
				return fmt.Errorf("%s: %w", it.tool.Name, err), nil
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

func runOne(ctx context.Context, dir string, t checks.Tool, det checks.DetectResult) error {
	cmd, err := checks.BuildCmd(ctx, dir, t, det)
	if err != nil {
		return err
	}
	return checks.RunAttached(cmd, dir)
}
