package materialyou

import (
	"math"
)

func isYellow(hue float64) bool {
	return hue >= 105.0 && hue < 125.0
}

func averageHex(h1, h2 string) string {
	a := hexToRgb(h1)
	b := hexToRgb(h2)
	avg := func(x, y float64) float64 {
		return math.Floor((x+y)/2.0 + 0.5)
	}
	return rgbToHex(RGB{
		R: avg(a.R, b.R),
		G: avg(a.G, b.G),
		B: avg(a.B, b.B),
	})
}

type TonalPalette struct {
	Hue    float64
	Chroma float64
	Tone   func(t float64) string
}

func newTonalPalette(hue, chroma float64) TonalPalette {
	var toneOf func(t float64) string
	toneOf = func(t float64) string {
		if t == 99.0 && isYellow(hue) {
			return averageHex(toneOf(98.0), toneOf(100.0))
		}
		return hexFromHct(HCT{Hue: hue, Chroma: chroma, Tone: t})
	}
	return TonalPalette{
		Hue:    hue,
		Chroma: chroma,
		Tone:   toneOf,
	}
}

type PaletteSet struct {
	Primary        TonalPalette
	Secondary      TonalPalette
	Tertiary       TonalPalette
	Neutral        TonalPalette
	NeutralVariant TonalPalette
	Error          TonalPalette
}

func corePalette(hue, chroma float64) PaletteSet {
	return PaletteSet{
		Primary:        newTonalPalette(hue, max(48.0, chroma)),
		Secondary:      newTonalPalette(hue, 16.0),
		Tertiary:       newTonalPalette(sanitizeDegrees(hue+60.0), 24.0),
		Neutral:        newTonalPalette(hue, 4.0),
		NeutralVariant: newTonalPalette(hue, 8.0),
		Error:          newTonalPalette(25.0, 84.0),
	}
}

func rainbowPalettes(hue, chroma float64) PaletteSet {
	return PaletteSet{
		Primary:        newTonalPalette(hue, 48.0),
		Secondary:      newTonalPalette(hue, 16.0),
		Tertiary:       newTonalPalette(sanitizeDegrees(hue+60.0), 24.0),
		Neutral:        newTonalPalette(hue, 0.0),
		NeutralVariant: newTonalPalette(hue, 0.0),
		Error:          newTonalPalette(25.0, 84.0),
	}
}
