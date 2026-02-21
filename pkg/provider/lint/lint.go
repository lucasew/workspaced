package lint

import (
	"context"
	"errors"
	"log/slog"

	"workspaced/pkg/provider"

	"github.com/owenrumney/go-sarif/v2/sarif"
)

// Linter extends the base Provider interface for static analysis tools.
type Linter interface {
	provider.Provider

	// Run executes the linter and returns a SARIF Run object.
	// If no issues are found, it may return an empty Run or nil.
	Run(ctx context.Context, dir string) (*sarif.Run, error)
}

// Register adds a linter to the global provider registry.
// This is typically called in init() functions of provider packages.
func Register(l Linter) {
	provider.Register[Linter](l)
}

// RunAll executes all globally registered linters against a directory and aggregates results.
func RunAll(ctx context.Context, dir string) (*sarif.Report, error) {
	report, err := sarif.New(sarif.Version210)
	if err != nil {
		return nil, err
	}

	// Retrieve all registered Linter implementations
	linters := provider.List[Linter]()

	for _, l := range linters {
		// 1. Check if the linter applies.
		err := l.Detect(ctx, dir)
		if errors.Is(err, provider.ErrNotApplicable) {
			slog.Debug("linter doesnt apply", "linter", l.Name())
			continue
		}
		if err != nil {
			slog.Debug("cant check if linter applies, skipping", "linter", l.Name(), "error", err)
			continue
		}

		// 2. Run the linter
		run, err := l.Run(ctx, dir)
		if err != nil {
			slog.Error("linter failed", "linter", l.Name(), "error", err)
			continue
		}

		// 3. Aggregate the result
		if run != nil {
			report.AddRun(run)
		}
	}

	return report, nil
}
