package ruff

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"

	"workspaced/pkg/checks"
	"workspaced/pkg/checks/lint"
	"workspaced/pkg/tool"

	"github.com/owenrumney/go-sarif/v2/sarif"
)

// check implements the lint.Linter interface for Python projects using Ruff.
type check struct{}

// New creates a new Ruff linter check.
func New() lint.Linter {
	return &check{}
}

func init() {
	lint.Register(New())
}

func (c *check) Name() string {
	return "ruff"
}

func (c *check) Detect(ctx context.Context, dir string) error {
	// Applies if uv.lock exists
	if _, err := os.Stat(filepath.Join(dir, "uv.lock")); os.IsNotExist(err) {
		return checks.ErrNotApplicable
	}
	return nil
}

func (c *check) Run(ctx context.Context, dir string) (*sarif.Run, error) {
	// Use tool.EnsureAndRun to execute ruff.
	// This automatically handles installation and version resolution.
	// Falls back to registry:ruff for the cataloged tool (with version prefix fixes).
	cmd, err := tool.EnsureAndRunLazyWithFallbackAt(ctx, dir, "ruff", "ruff", "registry:ruff", "check", "--output-format=sarif", "--exit-zero", ".")
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

	report, err := sarif.FromBytes(stdout.Bytes())
	if err != nil {
		return nil, fmt.Errorf("failed to parse sarif output: %w (stdout: %s)", err, stdout.String())
	}

	if len(report.Runs) > 0 {
		return report.Runs[0], nil
	}

	return nil, nil
}
