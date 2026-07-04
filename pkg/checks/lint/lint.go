package lint

import (
	"context"
	"errors"
	"fmt"
	"sync"

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
// and aggregates results.
func RunAll(ctx context.Context, dir string) (*sarif.Report, error) {
	logger := logging.GetLogger(ctx)
	report, err := sarif.New(sarif.Version210)
	if err != nil {
		return nil, err
	}

	linters := checks.List[Linter]()

	// Filter to applicable linters first (detect is cheap, run is expensive).
	var applicable []Linter
	for _, l := range linters {
		err := l.Detect(ctx, dir)
		if errors.Is(err, checks.ErrNotApplicable) {
			logger.Info("linter skipped", "linter", l.Name(), "reason", "not applicable")
			continue
		}
		if err != nil {
			logger.Warn("linter skipped", "linter", l.Name(), "reason", "detect failed", "error", err)
			continue
		}
		applicable = append(applicable, l)
	}

	if len(applicable) == 0 {
		return report, nil
	}

	// Run applicable linters in parallel using taskgroup.
	// The group must be present in the context (initialized only at the top
	// command and passed down).
	parent := taskgroup.MustFromContext(ctx)
	g, _ := parent.SubGroup(ctx)

	var mu sync.Mutex
	for _, l := range applicable {
		linter := l
		g.Go(fmt.Sprintf("lint:%s", linter.Name()), taskgroup.CPU, func(ctx context.Context, s *taskgroup.Status) error {
			l := logging.GetLogger(ctx)
			s.Update(fmt.Sprintf("running %s", linter.Name()))
			run, err := linter.Run(ctx, dir)
			if err != nil {
				logging.ReportError(ctx, err, "linter", linter.Name(), "context", "linter failed")
				return nil // Don't fail other linters.
			}
			resultCount := 0
			if run != nil {
				resultCount = len(run.Results)
			}
			l.Info("linter ok", "linter", linter.Name(), "sarif_results", resultCount)

			if run != nil {
				mu.Lock()
				report.AddRun(run)
				mu.Unlock()
			}
			return nil
		})
	}

	if err := g.Wait(); err != nil {
		return report, err
	}
	return report, nil
}
