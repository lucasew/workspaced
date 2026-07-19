package svgraster

import (
	"context"
	"image"

	"github.com/lucasew/workspaced/pkg/driver"
)

func Ensure(ctx context.Context) error {
	return driver.With(ctx, func(d Driver) error { return d.Ensure(ctx) })
}

func RasterizeSVG(ctx context.Context, svg string, width int, height int) (image.Image, error) {
	return driver.WithResult(ctx, func(d Driver) (image.Image, error) {
		return d.RasterizeSVG(ctx, svg, width, height)
	})
}
