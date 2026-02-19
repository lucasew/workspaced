package golangci

import (
	"bytes"
	"context"
	"os"
	"path/filepath"

	pkgexec "workspaced/pkg/driver/exec"
	"workspaced/pkg/provider/lint"

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
	// Applies if go.mod exists AND golangci-lint is available via the exec driver
	if _, err := os.Stat(filepath.Join(dir, "go.mod")); os.IsNotExist(err) {
		return false, nil
	}

	_, err := pkgexec.Which(ctx, "golangci-lint")
	return err == nil, nil
}

func (p *Provider) Run(ctx context.Context, dir string) (*sarif.Run, error) {
	// Execute golangci-lint run --out-format sarif using the exec driver
	cmd, err := pkgexec.Run(ctx, "golangci-lint", "run", "--out-format", "sarif")
	if err != nil {
		return nil, err
	}

	cmd.Dir = dir

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	// golangci-lint returns exit code 1 if issues are found.
	// We only return error if the command failed to run (e.g. binary not found)
	// AND we have no stdout to parse.
	if err := cmd.Run(); err != nil {
		if stdout.Len() == 0 {
			// Command failed completely (e.g. executable not found)
			return nil, err
		}
		// If we have stdout, it likely contains the SARIF report even with exit code 1
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
