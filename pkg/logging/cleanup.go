package logging

import (
	"context"
	"io"
)

// RunCleanup executes a cleanup action and reports any failure through ReportError.
// This is useful for deferred best-effort operations that must not mask primary errors.
// attrs are key/value pairs (same contract as ReportError).
func RunCleanup(ctx context.Context, op string, fn func() error, attrs ...any) {
	if fn == nil {
		return
	}
	if err := fn(); err != nil {
		reportWithOp(ctx, op, err, attrs...)
	}
}

// Close reports close failures through ReportError.
// attrs are key/value pairs (same contract as ReportError).
func Close(ctx context.Context, closer io.Closer, attrs ...any) {
	if closer == nil {
		return
	}
	RunCleanup(ctx, "close", closer.Close, attrs...)
}

type flusher interface {
	Flush() error
}

// Flush reports flush failures through ReportError.
// attrs are key/value pairs (same contract as ReportError).
func Flush(ctx context.Context, f flusher, attrs ...any) {
	if f == nil {
		return
	}
	RunCleanup(ctx, "flush", f.Flush, attrs...)
}

func reportWithOp(ctx context.Context, op string, err error, attrs ...any) {
	if err == nil {
		return
	}
	args := make([]any, 0, len(attrs)+2)
	if op != "" {
		args = append(args, "op", op)
	}
	args = append(args, attrs...)
	ReportError(ctx, err, args...)
}
