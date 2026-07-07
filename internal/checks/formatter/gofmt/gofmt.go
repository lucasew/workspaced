package gofmt

import (
	"context"

	"workspaced/internal/checks"
	"workspaced/internal/checks/formatter"
	"workspaced/pkg/driver/exec"
)

// check implements the formatter.Formatter interface for Go projects.
// It executes 'gofmt -w .' in the target directory.
type check struct{}

// New creates a new gofmt check.
func New() formatter.Formatter {
	return &check{}
}

func init() {
	formatter.Register(New())
}

func (c *check) Name() string {
	return "gofmt"
}

func (c *check) Detect(ctx context.Context, dir string) error {
	return checks.RequireFile(dir, "go.mod")
}

func (c *check) Format(ctx context.Context, dir string) error {
	cmd, err := exec.Run(ctx, "gofmt", "-w", ".")
	if err != nil {
		return err
	}
	return checks.RunAttached(cmd, dir)
}
