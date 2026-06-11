package ruff

import (
	"context"
	"os"
	"path/filepath"

	"workspaced/pkg/checks"
	"workspaced/pkg/checks/formatter"
	"workspaced/pkg/tool"
)

// Provider implements the formatter.Formatter interface for Python projects using Ruff.
type Provider struct{}

// New creates a new Ruff provider.
func New() formatter.Formatter {
	return &Provider{}
}

func init() {
	formatter.Register(New())
}

func (p *Provider) Name() string {
	return "ruff"
}

func (p *Provider) Detect(ctx context.Context, dir string) error {
	// Applies if uv.lock exists
	if _, err := os.Stat(filepath.Join(dir, "uv.lock")); os.IsNotExist(err) {
		return checks.ErrNotApplicable
	}
	return nil
}

func (p *Provider) Format(ctx context.Context, dir string) error {
	// Falls back to registry:ruff for the cataloged tool (with version prefix fixes).
	cmd, err := tool.EnsureAndRunLazyWithFallbackAt(ctx, dir, "ruff", "ruff", "registry:ruff", "format", ".")
	if err != nil {
		return err
	}
	cmd.Dir = dir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}
