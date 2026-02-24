package svgraster

import (
	"context"
	"image"
)

// Driver rasterizes SVG source code into image.Image.
type Driver interface {
	RasterizeSVG(ctx context.Context, svg string, width int, height int) (image.Image, error)
}
