package shell

import (
	"context"
	"workspaced/pkg/driver"
)

// Driver provides shell execution capabilities
type Driver interface {
	// Path returns the full path to the shell executable
	Path(ctx context.Context) (string, error)
}

// Get returns the active shell driver
func Get(ctx context.Context) (Driver, error) {
	return driver.Get[Driver](ctx)
}

// Path returns the path to the active shell
func Path(ctx context.Context) (string, error) {
	d, err := Get(ctx)
	if err != nil {
		return "", err
	}
	return d.Path(ctx)
}
