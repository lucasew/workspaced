package cmdctx

import (
	"context"
	"os"
	"strings"
)

type noCacheKey struct{}

// WithNoCache records whether full-cascade cold materialization is armed.
func WithNoCache(ctx context.Context, enabled bool) context.Context {
	return context.WithValue(ctx, noCacheKey{}, enabled)
}

// IsNoCache reports whether --no-cache / WORKSPACED_NO_CACHE is armed on ctx.
func IsNoCache(ctx context.Context) bool {
	v, _ := ctx.Value(noCacheKey{}).(bool)
	return v
}

// EnvNoCache reports whether WORKSPACED_NO_CACHE is set to a truthy value.
// Empty is off; 0/false/no/off (case-insensitive) are off; anything else is on.
func EnvNoCache() bool {
	v := strings.TrimSpace(os.Getenv("WORKSPACED_NO_CACHE"))
	if v == "" {
		return false
	}
	switch strings.ToLower(v) {
	case "0", "false", "no", "off":
		return false
	default:
		return true
	}
}
