package ruff

import (
	"context"

	"workspaced/pkg/checks"
	"workspaced/pkg/checks/formatter"
	"workspaced/pkg/tool"
)

// check implements the formatter.Formatter interface for Python projects using Ruff.
type check struct{}

// New creates a new Ruff check.
func New() formatter.Formatter {
	return &check{}
}

func init() {
	formatter.Register(New())
}

func (c *check) Name() string {
	return "ruff"
}

func (c *check) Detect(ctx context.Context, dir string) error {
	return checks.RequireFile(dir, "uv.lock")
}

func (c *check) Format(ctx context.Context, dir string) error {
	// Falls back to registry:ruff for the cataloged tool (with version prefix fixes).
	cmd, err := tool.EnsureAndRunLazyWithFallbackAt(ctx, dir, "ruff", "ruff", "registry:ruff", "format", ".")
	if err != nil {
		return err
	}
	return checks.RunAttached(cmd, dir)
}
