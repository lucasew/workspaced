package cmdctx

import "context"

type verboseKey struct{}

func WithVerbose(ctx context.Context, enabled bool) context.Context {
	return context.WithValue(ctx, verboseKey{}, enabled)
}

func IsVerbose(ctx context.Context) bool {
	v, _ := ctx.Value(verboseKey{}).(bool)
	return v
}

