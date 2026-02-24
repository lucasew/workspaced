package golangci

import (
	"bytes"
	"context"
	"log/slog"
	"os"
	"path/filepath"

	"workspaced/pkg/provider"
	"workspaced/pkg/provider/lint"
	"workspaced/pkg/tool"

	"github.com/owenrumney/go-sarif/v2/sarif"
)

// Provider implements the lint.Linter interface for Go projects.
// It executes 'golangci-lint' using the workspaced tool system.
type Provider struct{}

// New creates a new GolangCI-Lint provider.
func New() lint.Linter {
	return &Provider{}
}

func init() {
	lint.Register(New())
}

func (p *Provider) Name() string {
	return "golangci-lint"
}

func (p *Provider) Detect(ctx context.Context, dir string) error {
	// Applies if go.mod exists
	if _, err := os.Stat(filepath.Join(dir, "go.mod")); os.IsNotExist(err) {
		return provider.ErrNotApplicable
	}
	return nil
}

func (p *Provider) Run(ctx context.Context, dir string) (*sarif.Run, error) {
	// Use tool.EnsureAndRun to execute golangci-lint.
	// This automatically handles installation and version resolution.
	cmd, err := tool.EnsureAndRun(ctx, "github:golangci/golangci-lint@v1.64.6", "golangci-lint", "run", "--out-format=sarif", "--issues-exit-code=0")
	if err != nil {
		slog.Error("failed to setup golangci-lint", "err", err)
		return nil, err
	}

	cmd.Dir = dir

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		slog.Error("golangci-lint execution failed", "err", err, "stderr", stderr.String())
		return nil, err
	}

	// Parse SARIF output
	report, err := sarif.FromBytes(stdout.Bytes())
	if err != nil {
		return nil, err
	}

	if len(report.Runs) > 0 {
		return report.Runs[0], nil
	}

	return nil, nil
}
