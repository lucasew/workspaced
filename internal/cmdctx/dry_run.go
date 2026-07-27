package cmdctx

import "context"

type dryRunKey struct{}

func WithDryRun(ctx context.Context, enabled bool) context.Context {
	return context.WithValue(ctx, dryRunKey{}, enabled)
}

func IsDryRun(ctx context.Context) bool {
	v, ok := ctx.Value(dryRunKey{}).(bool)
	return ok && v
}
