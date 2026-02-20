package biome

import (
	"context"
	"os"
	"path/filepath"

	"workspaced/pkg/provider/formatter"
	"workspaced/pkg/tool"
)

// Provider implements the formatter.Formatter interface for Biome.
// It executes 'biome format --write' in the target directory.
type Provider struct{}

// New creates a new Biome provider.
func New() formatter.Formatter {
	return &Provider{}
}

func init() {
	formatter.Register(New())
}

func (p *Provider) Name() string {
	return "biome"
}

func (p *Provider) Detect(ctx context.Context, dir string) (bool, error) {
	// Applies if package.json exists
	if _, err := os.Stat(filepath.Join(dir, "package.json")); os.IsNotExist(err) {
		return false, nil
	}
	return true, nil
}

func (p *Provider) Format(ctx context.Context, dir string) error {
	// Use tool.EnsureAndRun to execute biome.
	// This automatically handles installation and version resolution.
	cmd, err := tool.EnsureAndRun(ctx, "github:biomejs/biome@@biomejs/biome@2.4.3", "biome", "format", "--write", ".")
	if err != nil {
		return err
	}

	cmd.Dir = dir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}
