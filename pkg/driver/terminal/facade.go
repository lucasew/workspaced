package terminal

import (
	"context"

	"github.com/lucasew/workspaced/pkg/driver"
)

// Open opens the preferred terminal emulator.
func Open(ctx context.Context, opts Options) error {
	return driver.With(ctx, func(d Driver) error { return d.Open(ctx, opts) })
}
