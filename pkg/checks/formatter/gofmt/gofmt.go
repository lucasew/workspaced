package gofmt

import (
	"context"
	"os"
	"path/filepath"

	"workspaced/pkg/checks"
	"workspaced/pkg/checks/formatter"
	"workspaced/pkg/driver/exec"
)

// Provider implements the formatter.Formatter interface for Go projects.
// It executes 'gofmt -w .' in the target directory.
type Provider struct{}

// New creates a new gofmt provider.
func New() formatter.Formatter {
	return &Provider{}
}

func init() {
	formatter.Register(New())
}

func (p *Provider) Name() string {
	return "gofmt"
}

func (p *Provider) Detect(ctx context.Context, dir string) error {
	// Applies if go.mod exists
	if _, err := os.Stat(filepath.Join(dir, "go.mod")); os.IsNotExist(err) {
		return checks.ErrNotApplicable
	}
	return nil
}

func (p *Provider) Format(ctx context.Context, dir string) error {
	cmd, err := exec.Run(ctx, "gofmt", "-w", ".")
	if err != nil {
		return err
	}
	cmd.Dir = dir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}
