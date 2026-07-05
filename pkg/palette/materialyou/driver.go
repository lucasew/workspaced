package materialyou

import (
	"context"
	"image"
	"sort"

	"workspaced/pkg/logging"
	"workspaced/pkg/palette/api"
)

type Driver struct{}

func (d *Driver) Name() string {
	return "materialyou"
}

func (d *Driver) Extract(ctx context.Context, img image.Image, opts api.Options) (*api.Palette, error) {
	logger := logging.GetLogger(ctx)

	colors := api.SampleImage(img, opts.MaxSamples)
	if len(colors) == 0 {
		return nil, ctx.Err()
	}

	freq := make(map[string]int)
	for _, c := range colors {
		hex := rgbToHex(RGB{R: float64(c.R), G: float64(c.G), B: float64(c.B)})
		freq[hex]++
	}

	type kv struct {
		k string
		v int
	}
	var ss []kv
	for k, v := range freq {
		ss = append(ss, kv{k, v})
	}
	sort.Slice(ss, func(i, j int) bool {
		return ss[i].v > ss[j].v
	})

	baseColor := ss[0].k
	logger.Info("selected base color for Material You", "hex", baseColor)

	hct := hctFromHex(baseColor)
	pal := &api.Palette{}

	// Simplified mapping using extracted HCT logic as a checkpoint.
	// The rest of the mapping will be added in a separate PR to maintain size limits.
	_ = hct
	return pal, nil
}
