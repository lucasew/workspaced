package driver

import (
	"context"
	"fmt"
	"os"
	"strings"

	"workspaced/pkg/executil"
)

// IsTermux reports whether TERMUX_VERSION is set in the process environment.
func IsTermux() bool {
	return os.Getenv("TERMUX_VERSION") != ""
}

// RequireTermux returns ErrIncompatible when not running under Termux.
func RequireTermux() error {
	if IsTermux() {
		return nil
	}
	return fmt.Errorf("%w: not running in Termux", ErrIncompatible)
}

// RequireEnv returns ErrIncompatible when key is unset or empty in ctx's env.
func RequireEnv(ctx context.Context, key string) error {
	if executil.GetEnv(ctx, key) != "" {
		return nil
	}
	return fmt.Errorf("%w: %s not set", ErrIncompatible, key)
}

// RequireAnyEnv returns ErrIncompatible when none of the keys are set.
func RequireAnyEnv(ctx context.Context, keys ...string) error {
	for _, key := range keys {
		if executil.GetEnv(ctx, key) != "" {
			return nil
		}
	}
	return fmt.Errorf("%w: none of %s set", ErrIncompatible, strings.Join(keys, ", "))
}
