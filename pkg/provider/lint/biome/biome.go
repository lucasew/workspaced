package biome

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
		return provider.ErrNotApplicable
	}
	return nil
}

func (p *Provider) Run(ctx context.Context, dir string) (*sarif.Run, error) {
	// Use tool.EnsureAndRun to execute biome.
	// This automatically handles installation and version resolution.
	cmd, err := tool.EnsureAndRun(ctx, "github:biomejs/biome@@biomejs/biome@2.4.3", "biome", "lint", "--reporter=sarif", ".")
	if err != nil {
		slog.Error("failed to setup biome", "err", err)
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
		slog.Debug("biome execution returned error (likely lint issues)", "err", runErr, "stderr", stderr.String())
	}

	// Parse SARIF output
	report, err := sarif.FromBytes(stdout.Bytes())
	if err != nil {
		slog.Error("failed to parse sarif output from biome", "err", err, "stdout", stdout.String())
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
