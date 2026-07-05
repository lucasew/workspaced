package materialyou

import (
	"context"
	"image"
	"slices"

	"workspaced/pkg/logging"
	"workspaced/pkg/palette/api"
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

	freq := make(map[string]int, len(colors))
	for _, c := range colors {
		freq[api.ToHex(c)]++
	}

	type kv struct {
		k string
		v int
	}
	ss := make([]kv, 0, len(freq))
	for k, v := range freq {
		ss = append(ss, kv{k, v})
	}
	// Frequency desc; hex asc on ties so map iteration order cannot flip the winner
	// (SampleImage returns unique colors, so multi-color images often all have count 1).
	slices.SortFunc(ss, func(a, b kv) int {
		if d := b.v - a.v; d != 0 {
			return d
		}
		if a.k < b.k {
			return -1
		}
		if a.k > b.k {
			return 1
		}
		return 0
	})

	baseColor := ss[0].k
	logger.Info("selected base color for Material You", "hex", "#"+baseColor)

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
