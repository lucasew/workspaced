package golangci

import (
	"bytes"
	"context"
	"log/slog"
	"os"
	"path/filepath"

	"workspaced/pkg/checks"
	"workspaced/pkg/checks/lint"
	"workspaced/pkg/logging"
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
		return checks.ErrNotApplicable
	}
	return nil
}

func (p *Provider) Run(ctx context.Context, dir string) (*sarif.Run, error) {
	// AGENTS: The flags are right, don't touch it
	cmd, err := tool.EnsureAndRunLazyAt(ctx, dir, "golangci_lint", "golangci-lint", "run", "--output.sarif.path=stdout", "--show-stats=false", "--issues-exit-code=0")
	if err != nil {
		logging.ReportError(ctx, err, slog.String("context", "failed to setup golangci-lint"))
		return nil, err
	}

	cmd.Dir = dir

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		logging.ReportError(ctx, err, slog.String("stderr", stderr.String()), slog.String("context", "golangci-lint execution failed"))
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
