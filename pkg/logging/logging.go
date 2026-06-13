package logging

import (
	"context"
	"encoding/json"
	"log/slog"

	"workspaced/pkg/types"
)

type loggerKey struct{}

// GetLogger retrieves the logger instance from the context.
// It panics if no logger has been injected into the context via ContextWithLogger.
// This enforces that all logging goes through a properly provided ctx
// (never a bare context.Background or context without a logger).
func GetLogger(ctx context.Context) *slog.Logger {
	if ctx == nil {
		panic("GetLogger called with nil context")
	}
	if logger, ok := ctx.Value(loggerKey{}).(*slog.Logger); ok && logger != nil {
		return logger
	}
	panic("no logger present in context; call ContextWithLogger on a root ctx before any GetLogger / ReportError / Close / RunCleanup etc. See cmd/workspaced/root.go for bootstrap pattern.")
}

// ContextWithLogger returns a context that carries the given *slog.Logger.
// This is the way to inject a (possibly derived) logger so that GetLogger
// and downstream code can retrieve it.
func ContextWithLogger(ctx context.Context, l *slog.Logger) context.Context {
	if l == nil {
		return ctx
	}
	return context.WithValue(ctx, loggerKey{}, l)
}

// NewRootContext returns a fresh root context (derived from context.Background)
// with the provided logger attached under LoggerKey. If l is nil, slog.Default()
// is used. This is the supported way to create the initial ctx for a process,
// command, test, or component when no parent ctx is available.
func NewRootContext(l *slog.Logger) context.Context {
	if l == nil {
		l = slog.Default()
	}
	return ContextWithLogger(context.Background(), l)
}

// ContextHasLogger reports whether ctx carries a non-nil logger under LoggerKey.
func ContextHasLogger(ctx context.Context) bool {
	if ctx == nil {
		return false
	}
	l, ok := ctx.Value(loggerKey{}).(*slog.Logger)
	return ok && l != nil
}

// ChannelLogHandler is a custom slog.Handler that broadcasts log records to a channel.
// This is used to stream server-side logs to the client via the daemon connection.
type ChannelLogHandler struct {
	Out    chan<- types.StreamPacket
	Parent slog.Handler
	Ctx    context.Context
}

// Enabled reports whether the handler handles records at the given level.
func (h *ChannelLogHandler) Enabled(ctx context.Context, level slog.Level) bool {
	return true
}

// Handle processes a log record, marshals it to JSON, and sends it as a StreamPacket.
// It also delegates to the parent handler if one is configured.
func (h *ChannelLogHandler) Handle(ctx context.Context, r slog.Record) error {
	entry := types.LogEntry{
		Level:   r.Level.String(),
		Message: r.Message,
		Attrs:   make(map[string]any),
	}
	r.Attrs(func(a slog.Attr) bool {
		entry.Attrs[a.Key] = a.Value.Any()
		return true
	})
	payload, _ := json.Marshal(entry)

	select {
	case h.Out <- types.StreamPacket{Type: "log", Payload: payload}:
	case <-h.Ctx.Done():
		return h.Ctx.Err()
	}

	if h.Parent != nil {
		return h.Parent.Handle(ctx, r)
	}
	return nil
}

// WithAttrs returns a new ChannelLogHandler with the given attributes added.
func (h *ChannelLogHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return &ChannelLogHandler{Out: h.Out, Parent: h.Parent.WithAttrs(attrs), Ctx: h.Ctx}
}

// WithGroup returns a new ChannelLogHandler with the given group name.
func (h *ChannelLogHandler) WithGroup(name string) slog.Handler {
	return &ChannelLogHandler{Out: h.Out, Parent: h.Parent.WithGroup(name), Ctx: h.Ctx}
}

// ReportError logs an unexpected error using the logger from the context.
// It serves as the centralized error reporting function.
func ReportError(ctx context.Context, err error, args ...any) bool {
	if err == nil {
		return false
	}
	logger := GetLogger(ctx)
	args = append(args, "error", err)
	logger.Error("unexpected error", args...)
	return true
}
