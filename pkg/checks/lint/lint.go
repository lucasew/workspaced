package lint

import (
	"context"
	"errors"
	"log/slog"

	"workspaced/pkg/checks"
	"workspaced/pkg/logging"

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

// RunAll executes all globally registered linters against a directory and aggregates results.
func RunAll(ctx context.Context, dir string) (*sarif.Report, error) {
	report, err := sarif.New(sarif.Version210)
	if err != nil {
		return nil, err
	}

	// Retrieve all registered Linter implementations
	linters := checks.List[Linter]()

	for _, l := range linters {
		// 1. Check if the linter applies.
		err := l.Detect(ctx, dir)
		if errors.Is(err, checks.ErrNotApplicable) {
			slog.Info("linter skipped", "linter", l.Name(), "reason", "not applicable")
			continue
		}
		if err != nil {
			slog.Warn("linter skipped", "linter", l.Name(), "reason", "detect failed", "error", err)
			continue
		}

		// 2. Run the linter
		run, err := l.Run(ctx, dir)
		if err != nil {
			logging.ReportError(ctx, err, slog.String("linter", l.Name()), slog.String("context", "linter failed"))
			continue
		}
		resultCount := 0
		if run != nil {
			resultCount = len(run.Results)
		}
		slog.Info("linter ok", "linter", l.Name(), "sarif_results", resultCount)

		// 3. Aggregate the result
		if run != nil {
			report.AddRun(run)
		}
	}

	return report, nil
}
