package shim

import (
	"context"
	"workspaced/pkg/driver"
)

// Driver provides shim script generation capabilities
type Driver interface {
	// GenerateContent creates the shim script content
	GenerateContent(command []string) (string, error)

	// Generate creates an executable shim at the given path that runs the specified command
	// path: absolute path where the shim will be created (e.g., "/home/user/.local/bin/python")
	// command: the command to execute when the shim is called (e.g., ["python3", "-m", "venv"])
	Generate(ctx context.Context, path string, command []string) error
}

// Get returns the active shim driver
func Get(ctx context.Context) (Driver, error) {
	return driver.Get[Driver](ctx)
}

// GenerateContent creates shim script content using the active driver
func GenerateContent(ctx context.Context, command []string) (string, error) {
	d, err := Get(ctx)
	if err != nil {
		return "", err
	}
	return d.GenerateContent(command)
}

// Generate creates a shim using the active driver
func Generate(ctx context.Context, path string, command []string) error {
	d, err := Get(ctx)
	if err != nil {
		return err
	}
	return d.Generate(ctx, path, command)
}
