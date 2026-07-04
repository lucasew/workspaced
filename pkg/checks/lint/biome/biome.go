package biome

import (
	"bytes"
	"context"

	"workspaced/pkg/checks"
	"workspaced/pkg/checks/lint"
	"workspaced/pkg/logging"
	"workspaced/pkg/tool"

	"github.com/owenrumney/go-sarif/v2/sarif"
)

// check implements the lint.Linter interface for Biome.
// It executes 'biome lint --reporter=sarif' in the target directory.
type check struct{}

// New creates a new Biome check.
func New() lint.Linter {
	return &check{}
}

func init() {
	lint.Register(New())
}

func (c *check) Name() string {
	return "biome"
}

func (c *check) Detect(ctx context.Context, dir string) error {
	return checks.RequireFile(dir, "package.json")
}

func (c *check) Run(ctx context.Context, dir string) (*sarif.Run, error) {
	// Use tool.EnsureAndRun to execute biome.
	// Falls back to registry:biome (catalog entry handles versions).
	cmd, err := tool.EnsureAndRunLazyWithFallbackAt(ctx, dir, "biome", "biome", "registry:biome", "lint", "--reporter=sarif", ".")
	if err != nil {
		logging.ReportError(ctx, err, "context", "setup biome")
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
		logger := logging.GetLogger(ctx)
		logger.Debug("biome execution returned error (likely lint issues)", "err", runErr, "stderr", stderr.String())
	}

	run, err := checks.FirstSARIFRun(stdout.Bytes())
	if err != nil {
		logging.ReportError(ctx, err, "stdout", stdout.String(), "context", "parse sarif output from biome")
		// If command failed and we couldn't parse SARIF, return the command error
		if runErr != nil {
			return nil, runErr
		}
		return nil, err
	}
	return run, nil
}
