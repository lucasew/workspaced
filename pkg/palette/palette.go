package palette

import (
	"context"
	"fmt"
	"image"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"os"
	"strings"

	"workspaced/pkg/logging"
	"workspaced/pkg/palette/api"
)

// GetDriver returns a palette extraction driver by name.
func GetDriver(ctx context.Context, name string) (api.Driver, error) {
	d, err := api.Get(name)
	if err != nil {
		available := api.Names()
		if len(available) == 0 {
			return nil, err
		}
		return nil, fmt.Errorf("%w (available: %s)", err, strings.Join(available, ", "))
	}
	return d, nil
}

// ListDrivers returns registered palette extraction drivers sorted by name.
func ListDrivers() []api.Driver {
	return api.List()
}

// DriverNames returns registered driver names in sorted order.
func DriverNames() []string {
	return api.Names()
}

// ExtractFromFile loads an image from a file and extracts a color palette.
func ExtractFromFile(ctx context.Context, path string, driver string, opts api.Options) (*api.Palette, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open image: %w", err)
	}
	defer logging.Close(ctx, f)

	img, _, err := image.Decode(f)
	if err != nil {
		return nil, fmt.Errorf("decode image: %w", err)
	}

	d, err := GetDriver(ctx, driver)
	if err != nil {
		return nil, err
	}

	return d.Extract(ctx, img, opts)
}
