package lint

import (
	"bytes"
	"context"
	"fmt"

	"workspaced/internal/checks"
	"workspaced/internal/checks/codec"
	"workspaced/pkg/logging"
	"workspaced/pkg/taskgroup"

	"github.com/owenrumney/go-sarif/v2/sarif"
)

// RunAll loads CUE lint tools for dir, runs applicable ones in parallel, bundles SARIF.
// A single tool failure is logged and omitted so siblings still contribute.
func RunAll(ctx context.Context, dir string) (*sarif.Report, error) {
	tools, err := checks.LoadToolsForDir(ctx, dir, "lint")
	if err != nil {
		return nil, err
	}
	type item struct {
		tool   checks.Tool
		detect checks.DetectResult
	}
	var applicable []item
	for _, t := range tools {
		if !t.Enable {
			continue
		}
		if t.Output == "" {
			logging.GetLogger(ctx).Warn("lint tool missing output codec; skipping", "tool", t.Name)
			continue
		}
		det, err := checks.EvaluateDetect(dir, t.Detect)
		if err != nil {
			logging.ReportError(ctx, err, "tool", t.Name, "context", "lint detect")
			continue
		}
		if !det.Applicable {
			logging.GetLogger(ctx).Debug("lint tool not applicable", "tool", t.Name)
			continue
		}
		applicable = append(applicable, item{tool: t, detect: det})
	}
	if len(applicable) == 0 {
		return checks.BundleRuns()
	}

	runs, err := taskgroup.Map[item, *sarif.Run]{
		Name:     "lint",
		Items:    applicable,
		PoolKind: taskgroup.CPU,
		TaskName: func(_ int, it item) string { return "lint:" + it.tool.Name },
		Fn: func(ctx context.Context, s *taskgroup.Status, it item) (*sarif.Run, error) {
			l := logging.GetLogger(ctx)
			s.Update("running " + it.tool.Name)
			run, err := runOne(ctx, dir, it.tool, it.detect)
			if err != nil {
				logging.ReportError(ctx, err, "linter", it.tool.Name, "context", "linter failed")
				return nil, nil
			}
			resultCount := 0
			if run != nil {
				resultCount = len(run.Results)
			}
			l.Info("linter ok", "linter", it.tool.Name, "sarif_results", resultCount)
			return run, nil
		},
	}.Run(ctx)
	if err != nil {
		return nil, err
	}
	return checks.BundleRuns(runs...)
}

func runOne(ctx context.Context, dir string, t checks.Tool, det checks.DetectResult) (*sarif.Run, error) {
	cmd, err := checks.BuildCmd(ctx, dir, t, det)
	if err != nil {
		return nil, err
	}
	stdout, stderr, runErr := checks.RunCapture(cmd)
	if runErr != nil && len(bytes.TrimSpace(stdout)) == 0 {
		return nil, fmt.Errorf("%s execution failed: %w (stderr: %s)", t.Name, runErr, string(stderr))
	}
	run, err := codec.Decode(t.Output, t.Name, stdout)
	if err != nil {
		if runErr != nil {
			return nil, fmt.Errorf("%s: %w (run: %v; stderr: %s)", t.Name, err, runErr, string(stderr))
		}
		return nil, fmt.Errorf("%s: %w", t.Name, err)
	}
	if run != nil && run.Tool.Driver.Name == "" {
		run.Tool.Driver.Name = t.Name
	}
	return run, nil
}
