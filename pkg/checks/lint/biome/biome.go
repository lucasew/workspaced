package biome

import (
	"bytes"
	"context"
	"os"
	"path/filepath"

	"workspaced/pkg/checks"
	"workspaced/pkg/checks/lint"
	"workspaced/pkg/logging"
	"workspaced/pkg/tool"

	"github.com/owenrumney/go-sarif/v2/sarif"
)

// Provider implements the lint.Linter interface for Biome.
// It executes 'biome lint --reporter=sarif' in the target directory.
type Provider struct{}

// New creates a new Biome provider.
func New() lint.Linter {
	return &Provider{}
}

func init() {
	lint.Register(New())
}

func (p *Provider) Name() string {
	return "biome"
}

func (p *Provider) Detect(ctx context.Context, dir string) error {
	// Applies if package.json exists
	if _, err := os.Stat(filepath.Join(dir, "package.json")); os.IsNotExist(err) {
		return checks.ErrNotApplicable
	}
	return nil
}

func (p *Provider) Run(ctx context.Context, dir string) (*sarif.Run, error) {
	// Use tool.EnsureAndRun to execute biome.
	// This automatically handles installation and version resolution.
	cmd, err := tool.EnsureAndRunLazyAt(ctx, dir, "biome", "biome", "lint", "--reporter=sarif", ".")
	if err != nil {
		logging.ReportError(ctx, err, "context", "failed to setup biome")
		return nil, err
	}

	cmd.Dir = dir

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	// biome returns non-zero exit code if lint issues are found.
	// We should try to parse SARIF output even if command fails.
	runErr := cmd.Run()
	if runErr != nil {
		logging.GetLogger(ctx).Debug("biome execution returned error (likely lint issues)", "err", runErr, "stderr", stderr.String())
	}

	// Parse SARIF output
	report, err := sarif.FromBytes(stdout.Bytes())
	if err != nil {
		logging.ReportError(ctx, err, "stdout", stdout.String(), "context", "failed to parse sarif output from biome")
		// If command failed and we couldn't parse SARIF, return the command error
		if runErr != nil {
			return nil, runErr
		}
		return nil, err
	}

	if len(report.Runs) > 0 {
		return report.Runs[0], nil
	}

	return nil, nil
}
