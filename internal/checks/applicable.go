package checks

import (
	"context"
	"errors"

	"github.com/lucasew/workspaced/pkg/logging"
)

// SkipFunc is called when a check is skipped during Applicable selection.
// reason is "not applicable" or "detect failed"; err is nil for the former.
type SkipFunc func(name, reason string, err error)

// LogSkip returns a SkipFunc that logs skips using the logger on ctx.
// not-applicable skips use Info; detect failures use Warn.
func LogSkip(ctx context.Context, kind string) SkipFunc {
	logger := logging.GetLogger(ctx)
	return func(name, reason string, err error) {
		if err != nil {
			logger.Warn(kind+" skipped", kind, name, "reason", reason, "error", err)
			return
		}
		logger.Info(kind+" skipped", kind, name, "reason", reason)
	}
}

// LogDetectFailures logs only Detect errors, silently ignoring not-applicable skips.
func LogDetectFailures(ctx context.Context, kind string) SkipFunc {
	logger := logging.GetLogger(ctx)
	return func(name, reason string, err error) {
		if err == nil {
			return
		}
		logger.Warn(kind+" detection failed", "name", name, "error", err)
	}
}

// Applicable returns the subset of checks whose Detect succeeds for dir.
// Skipped checks are reported via onSkip when non-nil.
func Applicable[T Check](ctx context.Context, dir string, items []T, onSkip SkipFunc) []T {
	if len(items) == 0 {
		return nil
	}
	out := make([]T, 0, len(items))
	for _, item := range items {
		err := item.Detect(ctx, dir)
		if err == nil {
			out = append(out, item)
			continue
		}
		if onSkip == nil {
			continue
		}
		if errors.Is(err, ErrNotApplicable) {
			onSkip(item.Name(), "not applicable", nil)
			continue
		}
		onSkip(item.Name(), "detect failed", err)
	}
	return out
}
