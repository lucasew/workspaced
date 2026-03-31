package tool

import (
	"context"
	"fmt"
	"os/exec"
)

// EnsureAndRunLazy resolves a lazy tool (respecting workspace config + lockfile),
// installs it if missing, and returns an exec.Cmd ready to run. This is the primary
// entry point for executing managed tools like linters or formatters.
func EnsureAndRunLazy(ctx context.Context, lazyName, binName string, args ...string) (*exec.Cmd, error) {
	binPath, err := ResolveLazyTool(ctx, lazyName, binName)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve lazy tool %q (%s): %w", lazyName, binName, err)
	}
	return exec.CommandContext(ctx, binPath, args...), nil
}

// EnsureAndRunLazyAt resolves a lazy tool using workspace detection strictly rooted at wd.
// It is specifically designed for codebase operations where the target directory (wd)
// differs from the process's working directory.
func EnsureAndRunLazyAt(ctx context.Context, wd, lazyName, binName string, args ...string) (*exec.Cmd, error) {
	binPath, err := ResolveLazyToolAt(ctx, wd, lazyName, binName)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve lazy tool %q (%s): %w", lazyName, binName, err)
	}
	return exec.CommandContext(ctx, binPath, args...), nil
}

// EnsureAndRunLazyWithFallback gracefully degrades tool execution. It attempts to resolve
// a tool via local/home configuration first. If unconfigured or unavailable, it falls back
// to installing/executing a hardcoded provider spec (fallbackSpec).
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
