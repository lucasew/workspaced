package govulncheck

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"

	"workspaced/pkg/checks"
	"workspaced/pkg/checks/lint"
	"workspaced/pkg/driver/exec"
	"workspaced/pkg/logging"

	"github.com/owenrumney/go-sarif/v2/sarif"
)

// check implements the lint.Linter interface for Go projects.
// It executes 'govulncheck' using the workspaced tool system.
type check struct{}

// New creates a new govulncheck check.
func New() lint.Linter {
	return &check{}
}

func init() {
	lint.Register(New())
}

func (c *check) Name() string {
	return "govulncheck"
}

func (c *check) Detect(ctx context.Context, dir string) error {
	// Applies if go.mod exists
	if _, err := os.Stat(filepath.Join(dir, "go.mod")); os.IsNotExist(err) {
		return checks.ErrNotApplicable
	}
	if exec.IsBinaryAvailable(ctx, "go") {
		return nil
	}
	return checks.ErrNotApplicable
}

func (c *check) Run(ctx context.Context, dir string) (*sarif.Run, error) {
	if !exec.IsBinaryAvailable(ctx, "go") {
		return nil, fmt.Errorf("%w: go binary not available for govulncheck", checks.ErrToolNotAvailable)
	}

	cmd, err := exec.Run(ctx, "go", "run", "golang.org/x/vuln/cmd/govulncheck@v1.1.4", "--format", "sarif", "./...")
	if err != nil {
		logging.ReportError(ctx, err, "context", "failed to setup govulncheck")
		return nil, err
	}

	cmd.Dir = dir

	var stdout bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		logging.ReportError(ctx, err, "context", "govulncheck execution failed")
		return nil, err
	}

	report, err := sarif.FromBytes(stdout.Bytes())
	if err != nil {
		return nil, err
	}

	if len(report.Runs) > 0 {
		return report.Runs[0], nil
	}

	return nil, nil
}
