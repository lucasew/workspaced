package govulncheck

import (
	"bytes"
	"context"
	"log/slog"
	"os"
	stdexec "os/exec"
	"path/filepath"

	"workspaced/pkg/driver/exec"
	"workspaced/pkg/provider/lint"

	"github.com/owenrumney/go-sarif/v2/sarif"
)

// Provider implements the lint.Linter interface for Go projects.
// It executes 'govulncheck' using the workspaced tool system.
type Provider struct{}

// New creates a new govulncheck provider.
func New() lint.Linter {
	return &Provider{}
}

func init() {
	lint.Register(New())
}

func (p *Provider) Name() string {
	return "govulncheck"
}

func (p *Provider) Detect(ctx context.Context, dir string) (bool, error) {
	// Applies if go.mod exists
	if _, err := os.Stat(filepath.Join(dir, "go.mod")); os.IsNotExist(err) {
		return false, nil
	}
	if exec.IsBinaryAvailable(ctx, "govulncheck") {
		return true, nil
	}
	if exec.IsBinaryAvailable(ctx, "go") {
		return true, nil
	}
	return false, nil
}

func (p *Provider) getCommand(ctx context.Context, dir string) (*stdexec.Cmd, error) {
	if exec.IsBinaryAvailable(ctx, "govulncheck") {
		return exec.Run(ctx, "govulncheck", "--format", "sarif", "./...")
	}
	if exec.IsBinaryAvailable(ctx, "go") {
		return exec.Run(ctx, "go", "run", "golang.org/x/vuln/cmd/govulncheck@latest", "--format", "sarif", "./...")
	}
	return nil, nil
}

func (p *Provider) Run(ctx context.Context, dir string) (*sarif.Run, error) {
	cmd, err := p.getCommand(ctx, dir)
	if err != nil || cmd == nil {
		slog.Error("failed to setup govulncheck", "err", err)
		return nil, err
	}

	cmd.Dir = dir

	var stdout bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		slog.Error("govulncheck execution failed", "err", err)
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
