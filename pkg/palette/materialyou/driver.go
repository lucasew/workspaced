package materialyou

import (
	"context"
	"image"
	"image/color"

	"github.com/lucasew/workspaced/pkg/logging"
	"github.com/lucasew/workspaced/pkg/palette/api"
)

func init() {
	api.Register(&Driver{})
}

type Driver struct{}

func (d *Driver) Name() string {
	return "materialyou"
}

func (d *Driver) Description() string {
	return "Material You tonal scheme from the dominant image color (Misterio77-style)"
}

func (d *Driver) Extract(ctx context.Context, img image.Image, opts api.Options) (*api.Palette, error) {
	logger := logging.GetLogger(ctx)

	colors := api.SampleImage(img, opts.MaxSamples)
	if len(colors) == 0 {
		return nil, ctx.Err()
	}

	// SampleImage returns unique colors (all frequency 1).
	// Pick the lexicographically smallest hex — equivalent to lowest packed RGB.
	minColor := colors[0]
	minKey := uint32(minColor.R)<<16 | uint32(minColor.G)<<8 | uint32(minColor.B)
	for _, c := range colors[1:] {
		key := uint32(c.R)<<16 | uint32(c.G)<<8 | uint32(c.B)
		if key < minKey {
			minKey = key
			minColor = c
		}
	}
	baseColor := api.ToHex(color.RGBA(minColor))
	// Debug: one-liner for CLI -d; avoid Info so default runs and tests stay quiet.
	logger.Debug("selected base color for Material You", "hex", "#"+baseColor)

	scheme := GenerateColorscheme("#"+baseColor, nil)

	var activeMode ModeColors
	if opts.Polarity == api.PolarityLight {
		activeMode = scheme.Light
	} else {
		activeMode = scheme.Dark
	}

	hexes := []string{
		activeMode["surface"],
		activeMode["surface_dim"],
		activeMode["surface_container"],
		activeMode["surface_container_high"],
		activeMode["surface_variant"],
		activeMode["on_surface"],
		activeMode["on_surface_variant"],
		activeMode["inverse_surface"],
		activeMode["error"],
		activeMode["secondary"],
		activeMode["tertiary"],
		activeMode["primary"],
		activeMode["primary_container"],
		activeMode["secondary_container"],
		activeMode["tertiary_container"],
		activeMode["error_container"],
	}
	if opts.ColorCount >= 24 {
		hexes = append(hexes,
			activeMode["surface_bright"],
			activeMode["surface_container_lowest"],
			activeMode["surface_container_low"],
			activeMode["surface_container_highest"],
			activeMode["outline"],
			activeMode["outline_variant"],
			activeMode["inverse_primary"],
			activeMode["inverse_on_surface"],
		)
	}

	return api.PaletteFromHexes(hexes), nil
}
