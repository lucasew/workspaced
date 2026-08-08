package lint

import (
	"context"

	"workspaced/pkg/checks"
	"workspaced/pkg/logging"
	"workspaced/pkg/taskgroup"

	"github.com/owenrumney/go-sarif/v2/sarif"
)

// Linter extends the base Check for static analysis tools.
type Linter interface {
	checks.Check

	// Run executes the linter and returns a SARIF Run object.
	// If no issues found, it may return an empty Run or nil.
	Run(ctx context.Context, dir string) (*sarif.Run, error)
}

// Register adds a linter to the global checks registry.
// This is typically called in init() functions of check implementations.
func Register(l Linter) {
	checks.Register[Linter](l)
}

// RunAll executes all globally registered linters in parallel against a directory
// and bundles each tool's SARIF run into one report (uber-SARIF).
// A single linter failure is logged and omitted from the bundle so other tools
// still contribute; only taskgroup-level errors fail the call.
func RunAll(ctx context.Context, dir string) (*sarif.Report, error) {
	applicable := checks.Applicable(ctx, dir, checks.List[Linter](), checks.LogSkip(ctx, "linter"))
	if len(applicable) == 0 {
		return checks.BundleRuns()
	}

	runs, err := taskgroup.Map[Linter, *sarif.Run]{
		Name:     "lint",
		Items:    applicable,
		PoolKind: taskgroup.CPU,
		TaskName: func(_ int, l Linter) string { return "lint:" + l.Name() },
		Fn: func(ctx context.Context, s *taskgroup.Status, linter Linter) (*sarif.Run, error) {
			l := logging.GetLogger(ctx)
			s.Update("running " + linter.Name())
			run, err := linter.Run(ctx, dir)
			if err != nil {
				logging.ReportError(ctx, err, "linter", linter.Name(), "context", "linter failed")
				return nil, nil // omit from bundle; keep siblings running
			}
			resultCount := 0
			if run != nil {
				resultCount = len(run.Results)
			}
			l.Info("linter ok", "linter", linter.Name(), "sarif_results", resultCount)
			return run, nil
		},
	}.Run(ctx)
	if err != nil {
		return nil, err
	}
	return checks.BundleRuns(runs...)
}
