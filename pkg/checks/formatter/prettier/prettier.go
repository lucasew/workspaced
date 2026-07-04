package prettier

import (
	"context"
	"errors"
	"fmt"

	"workspaced/pkg/checks"
	"workspaced/pkg/checks/formatter"
)

// check implements the formatter.Formatter interface for Prettier.
// It executes 'prettier --write .' in the target directory.
type check struct{}

// New creates a new Prettier check.
func New() formatter.Formatter {
	return &check{}
}

func init() {
	formatter.Register(New())
}

func (c *check) Name() string {
	return "prettier"
}

func (c *check) Detect(_ context.Context, dir string) error {
	return checks.RequireNodeModuleBin(dir, "prettier")
}

func (c *check) Format(ctx context.Context, dir string) error {
	cmd, err := checks.PrepareNodeModuleBin(ctx, dir, "prettier", "--write", ".")
	if err != nil {
		if errors.Is(err, checks.ErrToolNotAvailable) {
			return err
		}
		return fmt.Errorf("prepare prettier command: %w", err)
	}
	return checks.RunAttached(cmd, dir)
}
