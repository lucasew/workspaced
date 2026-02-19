package golangci

import (
	"bytes"
	"context"
	"log/slog"
	"os"
	"path/filepath"

	pkgexec "workspaced/pkg/driver/exec"
	"workspaced/pkg/provider/lint"
	"workspaced/pkg/tool"

	"github.com/owenrumney/go-sarif/v2/sarif"
)

// Provider implements the lint.Linter interface for Go projects.
// It executes 'golangci-lint' using the configured exec driver and parses its SARIF output.
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

func (p *Provider) Detect(ctx context.Context, dir string) (bool, error) {
	// Applies if go.mod exists AND mise is available
	if _, err := os.Stat(filepath.Join(dir, "go.mod")); os.IsNotExist(err) {
		return false, nil
	}

	_, err := pkgexec.Which(ctx, "mise")
	return err == nil, nil
}

func (p *Provider) Run(ctx context.Context, dir string) (*sarif.Run, error) {
	// Execute via mise: mise exec -- golangci-lint run --out-format=sarif --issues-exit-code=0
	// We use --issues-exit-code=0 so that lint issues don't cause the command to fail with exit code 1.
	// Only configuration errors or execution failures will return non-zero exit codes.
	cmd, err := tool.RunTool(ctx, "github:golangci/golangci-lint", "golangci-lint", "run", "--output.sarif.path=stdout", "--issues-exit-code=0")
	if err != nil {
		return nil, err
	}
	cmd.Dir = dir

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		// Since we used --issues-exit-code=0, any error here is a real execution failure
		slog.Error("golangci-lint execution failed", "err", err, "stderr", stderr.String())
		return nil, err
	}

	// Parse SARIF output

	report, err := sarif.FromBytes(stdout.Bytes())
	if err != nil {
		return nil, err
	}

	// golangci-lint produces one run per execution
	if len(report.Runs) > 0 {
		return report.Runs[0], nil
	}

	return nil, nil
}
