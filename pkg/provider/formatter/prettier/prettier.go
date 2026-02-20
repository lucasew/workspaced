package prettier

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"workspaced/pkg/driver/exec"
	"workspaced/pkg/provider/formatter"
)

// Provider implements the formatter.Formatter interface for Prettier.
// It executes 'prettier --write .' in the target directory.
type Provider struct{}

// New creates a new Prettier provider.
func New() formatter.Formatter {
	return &Provider{}
}

func init() {
	formatter.Register(New())
}

func (p *Provider) Name() string {
	return "prettier"
}

func (p *Provider) Detect(ctx context.Context, dir string) (bool, error) {
	// Applies if node_modules/.bin/prettier exists
	path := filepath.Join(dir, "node_modules", ".bin", "prettier")
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return false, nil
	}
	return true, nil
}

func (p *Provider) Format(ctx context.Context, dir string) error {
	binPath := filepath.Join(dir, "node_modules", ".bin", "prettier")

	if exec.IsBinaryAvailable(ctx, "node") {
		cmd, err := exec.Run(ctx, binPath, "--write", ".")
		if err != nil {
			return err
		}
		cmd.Dir = dir
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		return cmd.Run()
	}

	if exec.IsBinaryAvailable(ctx, "bun") {
		// If node is not available, try bun
		// "bun run --bun" forces bun runtime for the script
		cmd, err := exec.Run(ctx, "bun", "run", "--bun", binPath, "--write", ".")
		if err != nil {
			return err
		}
		cmd.Dir = dir
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		return cmd.Run()
	}

	return fmt.Errorf("neither node nor bun found in PATH, cannot run prettier")
}
