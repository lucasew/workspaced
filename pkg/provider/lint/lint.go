package lint

import (
	"context"

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

// Engine aggregates multiple Linters and runs them against a directory.
type Engine struct {
	linters []Linter
}

// NewEngine creates a new linter engine with the given linters.
func NewEngine(linters ...Linter) *Engine {
	return &Engine{linters: linters}
}

// Register adds a new linter to the engine.
func (e *Engine) Register(l Linter) {
	e.linters = append(e.linters, l)
}

// RunAll executes all applicable linters and aggregates their results into a single SARIF report.
func (e *Engine) RunAll(ctx context.Context, dir string) (*sarif.Report, error) {
	report, err := sarif.New(sarif.Version210)
	if err != nil {
		return nil, err
	}

	for _, l := range e.linters {
		// 1. Check if the linter applies
		applies, err := l.Detect(ctx, dir)
		if err != nil {
			// TODO: Add proper logging
			continue
		}
		if !applies {
			continue
		}

		// 2. Run the linter
		run, err := l.Run(ctx, dir)
		if err != nil {
			// TODO: Add proper logging
			continue
		}

		// 3. Aggregate the result
		if run != nil {
			report.AddRun(run)
		}
	}

	return report, nil
}
