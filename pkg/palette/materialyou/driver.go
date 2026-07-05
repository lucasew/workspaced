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

	scheme := GenerateColorscheme(baseColor, nil)

	var activeMode ModeColors
	if opts.Polarity == api.PolarityLight {
		activeMode = scheme.Light
	} else {
		activeMode = scheme.Dark
	}

	pal := &api.Palette{
		Base00: activeMode["surface"],
		Base01: activeMode["surface_dim"],
		Base02: activeMode["surface_container"],
		Base03: activeMode["surface_container_high"],
		Base04: activeMode["surface_variant"],
		Base05: activeMode["on_surface"],
		Base06: activeMode["on_surface_variant"],
		Base07: activeMode["inverse_surface"],
		Base08: activeMode["error"],
		Base09: activeMode["secondary"],
		Base0A: activeMode["tertiary"],
		Base0B: activeMode["primary"],
		Base0C: activeMode["primary_container"],
		Base0D: activeMode["secondary_container"],
		Base0E: activeMode["tertiary_container"],
		Base0F: activeMode["error_container"],
	}

	if opts.ColorCount >= 24 {
		pal.Base10 = activeMode["surface_bright"]
		pal.Base11 = activeMode["surface_container_lowest"]
		pal.Base12 = activeMode["surface_container_low"]
		pal.Base13 = activeMode["surface_container_highest"]
		pal.Base14 = activeMode["outline"]
		pal.Base15 = activeMode["outline_variant"]
		pal.Base16 = activeMode["inverse_primary"]
		pal.Base17 = activeMode["inverse_on_surface"]
	}

	stripHash := func(s string) string {
		if len(s) > 0 && s[0] == '#' {
			return s[1:]
		}
		return s
	}

	pal.Base00 = stripHash(pal.Base00)
	pal.Base01 = stripHash(pal.Base01)
	pal.Base02 = stripHash(pal.Base02)
	pal.Base03 = stripHash(pal.Base03)
	pal.Base04 = stripHash(pal.Base04)
	pal.Base05 = stripHash(pal.Base05)
	pal.Base06 = stripHash(pal.Base06)
	pal.Base07 = stripHash(pal.Base07)
	pal.Base08 = stripHash(pal.Base08)
	pal.Base09 = stripHash(pal.Base09)
	pal.Base0A = stripHash(pal.Base0A)
	pal.Base0B = stripHash(pal.Base0B)
	pal.Base0C = stripHash(pal.Base0C)
	pal.Base0D = stripHash(pal.Base0D)
	pal.Base0E = stripHash(pal.Base0E)
	pal.Base0F = stripHash(pal.Base0F)
	pal.Base10 = stripHash(pal.Base10)
	pal.Base11 = stripHash(pal.Base11)
	pal.Base12 = stripHash(pal.Base12)
	pal.Base13 = stripHash(pal.Base13)
	pal.Base14 = stripHash(pal.Base14)
	pal.Base15 = stripHash(pal.Base15)
	pal.Base16 = stripHash(pal.Base16)
	pal.Base17 = stripHash(pal.Base17)

	return pal, nil
}
