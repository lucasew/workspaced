package source

import "context"

type warningSinkKey struct{}

// WithWarningSink attaches a pointer that plugins may append soft diagnostics to.
// Safe when sink is non-nil; AppendWarning no-ops if no sink is present.
func WithWarningSink(ctx context.Context, sink *[]string) context.Context {
	if sink == nil {
		return ctx
	}
	return context.WithValue(ctx, warningSinkKey{}, sink)
}

// AppendWarning records a soft diagnostic when a sink is on ctx.
func AppendWarning(ctx context.Context, msg string) {
	if msg == "" {
		return
	}
	sink, ok := ctx.Value(warningSinkKey{}).(*[]string)
	if !ok || sink == nil {
		return
	}
	*sink = append(*sink, msg)
}
