package svgraster

import (
	"context"
	"image"
	"workspaced/pkg/driver"
)

func RasterizeSVG(ctx context.Context, svg string, width int, height int) (image.Image, error) {
	d, err := driver.Get[Driver](ctx)
	if err != nil {
		return nil, err
	}
	return d.RasterizeSVG(ctx, svg, width, height)
}
