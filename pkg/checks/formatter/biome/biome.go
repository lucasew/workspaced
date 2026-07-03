package biome

import (
	"context"
	"os"
	"path/filepath"

	"workspaced/pkg/checks"
	"workspaced/pkg/checks/formatter"
	"workspaced/pkg/tool"
)

// check implements the formatter.Formatter interface for Biome.
// It executes 'biome format --write' in the target directory.
type check struct{}

// New creates a new Biome check.
func New() formatter.Formatter {
	return &check{}
}

func init() {
	formatter.Register(New())
}

func (c *check) Name() string {
	return "biome"
}

func (c *check) Detect(ctx context.Context, dir string) error {
	// Applies if package.json exists
	if _, err := os.Stat(filepath.Join(dir, "package.json")); os.IsNotExist(err) {
		return checks.ErrNotApplicable
	}
	return nil
}

func (c *check) Format(ctx context.Context, dir string) error {
	// Use tool.EnsureAndRun to execute biome.
	// Falls back to registry:biome (catalog entry handles versions).
	cmd, err := tool.EnsureAndRunLazyWithFallbackAt(ctx, dir, "biome", "biome", "registry:biome", "format", "--write", ".")
	if err != nil {
		return err
	}

	cmd.Dir = dir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}
