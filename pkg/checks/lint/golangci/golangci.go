package golangci

import (
	"bytes"
	"context"

	"workspaced/pkg/checks"
	"workspaced/pkg/checks/lint"
	"workspaced/pkg/logging"
	"workspaced/pkg/tool"

	"github.com/owenrumney/go-sarif/v2/sarif"
)

// check implements the lint.Linter interface for Go projects.
// It executes 'golangci-lint' using the workspaced tool system.
type check struct{}

// New creates a new GolangCI-Lint check.
func New() lint.Linter {
	return &check{}
}

func init() {
	lint.Register(New())
}

func (c *check) Name() string {
	return "golangci-lint"
}

func (c *check) Detect(ctx context.Context, dir string) error {
	return checks.RequireFile(dir, "go.mod")
}

func (c *check) Run(ctx context.Context, dir string) (*sarif.Run, error) {
	// AGENTS: The flags are right, don't touch it
	// Falls back to registry:golangci-lint (the catalog entry normalizes v-prefixes on GitHub tags).
	cmd, err := tool.EnsureAndRunLazyWithFallbackAt(ctx, dir, "golangci_lint", "golangci-lint", "registry:golangci-lint", "run", "--output.sarif.path=stdout", "--show-stats=false", "--issues-exit-code=0")
	if err != nil {
		logging.ReportError(ctx, err, "context", "setup golangci-lint")
		return nil, err
	}

	cmd.Dir = dir

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		logging.ReportError(ctx, err, "stderr", stderr.String(), "context", "golangci-lint execution failed")
		return nil, err
	}

	return checks.FirstSARIFRun(stdout.Bytes())
}
