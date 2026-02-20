package ruff

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"

	"workspaced/pkg/provider/lint"
	"workspaced/pkg/tool"

	"github.com/owenrumney/go-sarif/v2/sarif"
)

// Provider implements the lint.Linter interface for Python projects using Ruff.
type Provider struct{}

// New creates a new Ruff linter provider.
func New() lint.Linter {
	return &Provider{}
}

func init() {
	lint.Register(New())
}

func (p *Provider) Name() string {
	return "ruff"
}

func (p *Provider) Detect(ctx context.Context, dir string) (bool, error) {
	// Applies if uv.lock exists
	if _, err := os.Stat(filepath.Join(dir, "uv.lock")); os.IsNotExist(err) {
		return false, nil
	}
	return true, nil
}

func (p *Provider) Run(ctx context.Context, dir string) (*sarif.Run, error) {
	// Use tool.EnsureAndRun to execute ruff.
	// This automatically handles installation and version resolution.
	cmd, err := tool.EnsureAndRun(ctx, "github:astral-sh/ruff@0.6.2", "ruff", "check", "--output-format=sarif", "--exit-zero", ".")
	if err != nil {
		return nil, fmt.Errorf("failed to setup ruff: %w", err)
	}

	cmd.Dir = dir

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("ruff execution failed: %w (stderr: %s)", err, stderr.String())
	}

	// Parse SARIF output
	report, err := sarif.FromBytes(stdout.Bytes())
	if err != nil {
		return nil, fmt.Errorf("failed to parse sarif output: %w (stdout: %s)", err, stdout.String())
	}

	if len(report.Runs) > 0 {
		return report.Runs[0], nil
	}

	return nil, nil
}
