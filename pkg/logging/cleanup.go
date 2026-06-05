package logging

import (
	"context"
	"io"
	"log/slog"
)

// RunCleanup executes a cleanup action and reports any failure through ReportError.
// This is useful for deferred best-effort operations that must not mask primary errors.
func RunCleanup(ctx context.Context, op string, fn func() error, attrs ...slog.Attr) {
	if fn == nil {
		return
	}
	if err := fn(); err != nil {
		reportWithOp(ctx, op, err, attrs...)
	}
}

// Close reports close failures through ReportError.
func Close(ctx context.Context, closer io.Closer, attrs ...slog.Attr) {
	if closer == nil {
		return
	}
	RunCleanup(ctx, "close", closer.Close, attrs...)
}

type flusher interface {
	Flush() error
}

// Flush reports flush failures through ReportError.
func Flush(ctx context.Context, f flusher, attrs ...slog.Attr) {
	if f == nil {
		return
	}
	RunCleanup(ctx, "flush", f.Flush, attrs...)
}

func reportWithOp(ctx context.Context, op string, err error, attrs ...slog.Attr) {
	if err == nil {
		return
	}
	args := make([]slog.Attr, 0, len(attrs)+1)
	if op != "" {
		args = append(args, slog.String("op", op))
	}
	args = append(args, attrs...)
	ReportError(ctx, err, args...)
}
