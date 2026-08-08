package clipboard

import (
	"context"
	"image"

	"workspaced/pkg/driver"
)

// WriteImage writes a stdlib image.Image to the clipboard using the available driver.
func WriteImage(ctx context.Context, img image.Image) error {
	return driver.With(ctx, func(d Driver) error { return d.WriteImage(ctx, img) })
}

// WriteText writes text to the clipboard using the available driver.
func WriteText(ctx context.Context, text string) error {
	return driver.With(ctx, func(d Driver) error { return d.WriteText(ctx, text) })
}
