package tool

import (
	"context"
	"fmt"
	"os/exec"
)

// EnsureAndRunLazy resolves a lazy tool (respecting workspace config + lockfile)
// and returns an exec.Cmd ready to run.
func EnsureAndRunLazy(ctx context.Context, lazyName, binName string, args ...string) (*exec.Cmd, error) {
	binPath, err := ResolveLazyTool(ctx, lazyName, binName)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve lazy tool %q (%s): %w", lazyName, binName, err)
	}
	return exec.CommandContext(ctx, binPath, args...), nil
}

// EnsureAndRunLazyAt resolves a lazy tool using workspace detection rooted at wd.
func EnsureAndRunLazyAt(ctx context.Context, wd, lazyName, binName string, args ...string) (*exec.Cmd, error) {
	binPath, err := ResolveLazyToolAt(ctx, wd, lazyName, binName)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve lazy tool %q (%s): %w", lazyName, binName, err)
	}
	return exec.CommandContext(ctx, binPath, args...), nil
}

// EnsureAndRunLazyWithFallback first tries lazy resolution, then falls back to
// an explicit tool spec to preserve behavior when lazy_tools is not configured.
func EnsureAndRunLazyWithFallback(ctx context.Context, lazyName, binName, fallbackSpec string, args ...string) (*exec.Cmd, error) {
	cmd, err := EnsureAndRunLazy(ctx, lazyName, binName, args...)
	if err == nil {
		return cmd, nil
	}
	if fallbackSpec == "" {
		return nil, err
	}
	return EnsureAndRun(ctx, fallbackSpec, binName, args...)
}
