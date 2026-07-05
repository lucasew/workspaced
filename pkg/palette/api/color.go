package api

import (
	"fmt"
	"image"
	"image/color"
	"math"
	"strings"
)

// NormalizeHex returns s without a leading '#'. Empty input stays empty.
func NormalizeHex(s string) string {
	return strings.TrimPrefix(s, "#")
}

// PaletteFromHexes builds a Palette from hex strings (with or without '#').
// At most 24 entries are used; missing entries leave the corresponding field empty.
func PaletteFromHexes(hexes []string) *Palette {
	p := &Palette{}
	slots := []*string{
		&p.Base00, &p.Base01, &p.Base02, &p.Base03,
		&p.Base04, &p.Base05, &p.Base06, &p.Base07,
		&p.Base08, &p.Base09, &p.Base0A, &p.Base0B,
		&p.Base0C, &p.Base0D, &p.Base0E, &p.Base0F,
		&p.Base10, &p.Base11, &p.Base12, &p.Base13,
		&p.Base14, &p.Base15, &p.Base16, &p.Base17,
	}
	n := min(len(hexes), len(slots))
	for i := range n {
		*slots[i] = NormalizeHex(hexes[i])
	}
	return p
}

// RGBToLAB converts RGB color to LAB for perceptual distance calculations.
// Based on Stylix Data/Colour.hs rgb2lab.
func RGBToLAB(c color.Color) LAB {
	r, g, b, _ := c.RGBA()

	rf := float64(r) / 65535.0
	gf := float64(g) / 65535.0
	bf := float64(b) / 65535.0

	rf = gammaCorrect(rf)
	gf = gammaCorrect(gf)
	bf = gammaCorrect(bf)

	x := rf*0.4124564 + gf*0.3575761 + bf*0.1804375
	y := rf*0.2126729 + gf*0.7151522 + bf*0.0721750
	z := rf*0.0193339 + gf*0.1191920 + bf*0.9503041

	x /= 0.95047
	y /= 1.00000
	z /= 1.08883

	x = labF(x)
	y = labF(y)
	z = labF(z)

	return LAB{
		L: 116.0*y - 16.0,
		A: 500.0 * (x - y),
		B: 200.0 * (y - z),
	}
}

func gammaCorrect(v float64) float64 {
	if v <= 0.04045 {
		return v / 12.92
	}
	return math.Pow((v+0.055)/1.055, 2.4)
}

func labF(t float64) float64 {
	delta := 6.0 / 29.0
	if t > delta*delta*delta {
		return math.Pow(t, 1.0/3.0)
	}
	return t/(3.0*delta*delta) + 4.0/29.0
}

// LABToRGB converts LAB back to RGB.
// Based on Stylix Data/Colour.hs lab2rgb.
func LABToRGB(lab LAB) color.RGBA {
	fy := (lab.L + 16.0) / 116.0
	fx := lab.A/500.0 + fy
	fz := fy - lab.B/200.0

	x := labFInv(fx) * 0.95047
	y := labFInv(fy) * 1.00000
	z := labFInv(fz) * 1.08883

	r := x*3.2404542 + y*-1.5371385 + z*-0.4985314
	g := x*-0.9692660 + y*1.8760108 + z*0.0415560
	b := x*0.0556434 + y*-0.2040259 + z*1.0572252

	r = gammaInverse(r)
	g = gammaInverse(g)
	b = gammaInverse(b)

	return color.RGBA{
		R: uint8(clamp(r*255.0, 0, 255)),
		G: uint8(clamp(g*255.0, 0, 255)),
		B: uint8(clamp(b*255.0, 0, 255)),
		A: 255,
	}
}

func labFInv(t float64) float64 {
	delta := 6.0 / 29.0
	if t > delta {
		return t * t * t
	}
	return 3.0 * delta * delta * (t - 4.0/29.0)
}

func gammaInverse(v float64) float64 {
	if v <= 0.0031308 {
		return v * 12.92
	}
	return 1.055*math.Pow(v, 1.0/2.4) - 0.055
}

func clamp(v, lo, hi float64) float64 {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

// DeltaE calculates perceptual color distance (CIE76).
func DeltaE(c1, c2 LAB) float64 {
	dl := c1.L - c2.L
	da := c1.A - c2.A
	db := c1.B - c2.B
	return math.Sqrt(dl*dl + da*da + db*db)
}

// ToHex converts color.Color to lowercase RRGGBB without '#'.
func ToHex(c color.Color) string {
	r, g, b, _ := c.RGBA()
	return fmt.Sprintf("%02x%02x%02x", uint8(r>>8), uint8(g>>8), uint8(b>>8))
}

// Lightness extracts CIELAB L* from a color.
func Lightness(c color.Color) float64 {
	return RGBToLAB(c).L
}

// SampleImage extracts unique opaque colors from img.
// When maxSamples > 0 and the image is larger, pixels are strided so at most
// about maxSamples positions are visited.
func SampleImage(img image.Image, maxSamples int) []color.RGBA {
	bounds := img.Bounds()
	width := bounds.Dx()
	height := bounds.Dy()
	totalPixels := width * height

	stride := 1
	if maxSamples > 0 && totalPixels > maxSamples {
		stride = int(math.Ceil(float64(totalPixels) / float64(maxSamples)))
	}

	colorMap := make(map[uint32]color.RGBA)
	for y := bounds.Min.Y; y < bounds.Max.Y; y += stride {
		for x := bounds.Min.X; x < bounds.Max.X; x += stride {
			c := img.At(x, y)
			r, g, b, a := c.RGBA()
			if a == 0 {
				continue
			}
			rgba := color.RGBA{
				R: uint8(r >> 8),
				G: uint8(g >> 8),
				B: uint8(b >> 8),
				A: 255,
			}
			key := uint32(rgba.R)<<16 | uint32(rgba.G)<<8 | uint32(rgba.B)
			colorMap[key] = rgba
		}
	}

	colors := make([]color.RGBA, 0, len(colorMap))
	for _, c := range colorMap {
		colors = append(colors, c)
	}
	return colors
}
