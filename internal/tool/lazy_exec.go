package tool

import (
	"context"
	"fmt"
	"os/exec"
	execdriver "github.com/lucasew/workspaced/pkg/driver/exec"
)

// EnsureAndRunLazy handles the lifecycle for a tool configured dynamically in a workspace.
// It resolves the alias against the workspace configuration, checks the lockfile for version
// pinning to ensure reproducibility, downloads if necessary, and returns a configured *exec.Cmd.
func EnsureAndRunLazy(ctx context.Context, lazyName, binName string, args ...string) (*exec.Cmd, error) {
	binPath, err := ResolveLazyTool(ctx, lazyName, binName)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve lazy tool %q (%s): %w", lazyName, binName, err)
	}
	return execdriver.Run(ctx, binPath, args...)
}

// EnsureAndRunLazyAt mirrors EnsureAndRunLazy but forces workspace detection to anchor
// at a specific directory (wd) rather than the default process context.
func EnsureAndRunLazyAt(ctx context.Context, wd, lazyName, binName string, args ...string) (*exec.Cmd, error) {
	binPath, err := ResolveLazyToolAt(ctx, wd, lazyName, binName)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve lazy tool %q (%s): %w", lazyName, binName, err)
	}
	return execdriver.Run(ctx, binPath, args...)
}

// EnsureAndRunLazyWithFallback attempts to resolve a tool via workspace configuration first.
// If the tool alias is unmapped in the workspace, it falls back to installing and executing
// an explicitly provided universal tool spec (e.g. "github:cli/cli@latest"), bridging dynamic
// environments with global defaults.
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

// EnsureAndRunLazyWithFallbackAt is like EnsureAndRunLazyWithFallback but forces
// workspace detection to anchor at a specific directory (wd).
func EnsureAndRunLazyWithFallbackAt(ctx context.Context, wd, lazyName, binName, fallbackSpec string, args ...string) (*exec.Cmd, error) {
	cmd, err := EnsureAndRunLazyAt(ctx, wd, lazyName, binName, args...)
	if err == nil {
		return cmd, nil
	}
	if fallbackSpec == "" {
		return nil, err
	}
	return EnsureAndRun(ctx, fallbackSpec, binName, args...)
}
