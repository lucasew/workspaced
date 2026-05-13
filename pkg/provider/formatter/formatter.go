package formatter

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	"workspaced/pkg/provider"
)

// Formatter extends the base Provider interface for code formatting tools.
type Formatter interface {
	provider.Provider

	// Format applies formatting changes to files in the directory.
	Format(ctx context.Context, dir string) error
}

// Register adds a formatter to the global registry.
func Register(f Formatter) {
	provider.Register[Formatter](f)
}

// RunAll executes all applicable formatters.
func RunAll(ctx context.Context, dir string) error {
	formatters := provider.List[Formatter]()
	slog.Info("Running formatters", "count", len(formatters), "dir", dir)

	var errs []error

	for _, f := range formatters {
		err := f.Detect(ctx, dir)
		if errors.Is(err, provider.ErrNotApplicable) {
			continue
		}
		if err != nil {
			slog.Warn("formatter detection failed", "name", f.Name(), "error", err)
			continue
		}

		slog.Info("Running formatter", "name", f.Name())
		if err := f.Format(ctx, dir); err != nil {
			slog.Error("formatter failed", "name", f.Name(), "error", err)
			errs = append(errs, fmt.Errorf("%s: %w", f.Name(), err))
		}
	}

	if len(errs) > 0 {
		return fmt.Errorf("formatting failed for %d tools: %v", len(errs), errs)
	}
	return nil
}
