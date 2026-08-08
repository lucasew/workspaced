package svgraster

import (
	"context"
	"image"
)

// Driver rasterizes SVG source code into image.Image.
type Driver interface {
	// Ensure ensures the underlying rasterizer (e.g. resvg) is installed and
	// ready. This is useful to trigger tool resolution/install (which may be
	// network-heavy and benefit from progress reporting) before starting
	// expensive parallel work that will call RasterizeSVG.
	Ensure(ctx context.Context) error

	RasterizeSVG(ctx context.Context, svg string, width int, height int) (image.Image, error)
}
