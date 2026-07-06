package api

import (
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

const labDelta = 6.0 / 29.0
const labDelta3 = labDelta * labDelta * labDelta
const labDeltaSq3 = 3.0 * labDelta * labDelta

func labF(t float64) float64 {
	if t > labDelta3 {
		return math.Cbrt(t)
	}
	return t/labDeltaSq3 + 4.0/29.0
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

var hexLUT = [256]string{
	"00", "01", "02", "03", "04", "05", "06", "07", "08", "09", "0a", "0b", "0c", "0d", "0e", "0f",
	"10", "11", "12", "13", "14", "15", "16", "17", "18", "19", "1a", "1b", "1c", "1d", "1e", "1f",
	"20", "21", "22", "23", "24", "25", "26", "27", "28", "29", "2a", "2b", "2c", "2d", "2e", "2f",
	"30", "31", "32", "33", "34", "35", "36", "37", "38", "39", "3a", "3b", "3c", "3d", "3e", "3f",
	"40", "41", "42", "43", "44", "45", "46", "47", "48", "49", "4a", "4b", "4c", "4d", "4e", "4f",
	"50", "51", "52", "53", "54", "55", "56", "57", "58", "59", "5a", "5b", "5c", "5d", "5e", "5f",
	"60", "61", "62", "63", "64", "65", "66", "67", "68", "69", "6a", "6b", "6c", "6d", "6e", "6f",
	"70", "71", "72", "73", "74", "75", "76", "77", "78", "79", "7a", "7b", "7c", "7d", "7e", "7f",
	"80", "81", "82", "83", "84", "85", "86", "87", "88", "89", "8a", "8b", "8c", "8d", "8e", "8f",
	"90", "91", "92", "93", "94", "95", "96", "97", "98", "99", "9a", "9b", "9c", "9d", "9e", "9f",
	"a0", "a1", "a2", "a3", "a4", "a5", "a6", "a7", "a8", "a9", "aa", "ab", "ac", "ad", "ae", "af",
	"b0", "b1", "b2", "b3", "b4", "b5", "b6", "b7", "b8", "b9", "ba", "bb", "bc", "bd", "be", "bf",
	"c0", "c1", "c2", "c3", "c4", "c5", "c6", "c7", "c8", "c9", "ca", "cb", "cc", "cd", "ce", "cf",
	"d0", "d1", "d2", "d3", "d4", "d5", "d6", "d7", "d8", "d9", "da", "db", "dc", "dd", "de", "df",
	"e0", "e1", "e2", "e3", "e4", "e5", "e6", "e7", "e8", "e9", "ea", "eb", "ec", "ed", "ee", "ef",
	"f0", "f1", "f2", "f3", "f4", "f5", "f6", "f7", "f8", "f9", "fa", "fb", "fc", "fd", "fe", "ff",
}

// ToHex converts color.Color to lowercase RRGGBB without '#'.
func ToHex(c color.Color) string {
	r, g, b, _ := c.RGBA()
	return hexLUT[uint8(r>>8)] + hexLUT[uint8(g>>8)] + hexLUT[uint8(b>>8)]
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

	// Pre-allocate only for full scans where many unique colors are expected.
	// Capped scans typically yield far fewer unique colors than sampled pixels.
	var colorSet map[uint32]struct{}
	if maxSamples <= 0 {
		colorSet = make(map[uint32]struct{}, totalPixels)
	} else {
		colorSet = make(map[uint32]struct{})
	}

	// Type-switch on concrete image types to avoid interface boxing on At().
	switch src := img.(type) {
	case *image.NRGBA:
		sampleNRGBA(src, bounds, stride, colorSet)
	case *image.RGBA:
		sampleRGBA(src, bounds, stride, colorSet)
	case *image.Paletted:
		samplePaletted(src, bounds, stride, colorSet)
	default:
		sampleGeneric(img, bounds, stride, colorSet)
	}

	colors := make([]color.RGBA, 0, len(colorSet))
	for key := range colorSet {
		colors = append(colors, color.RGBA{
			R: uint8(key >> 16),
			G: uint8(key >> 8),
			B: uint8(key),
			A: 255,
		})
	}
	return colors
}

func sampleNRGBA(src *image.NRGBA, bounds image.Rectangle, stride int, colorSet map[uint32]struct{}) {
	pix := src.Pix
	imgStride := src.Stride
	for y := bounds.Min.Y; y < bounds.Max.Y; y += stride {
		rowOff := (y-bounds.Min.Y)*imgStride + (0)*4
		for x := bounds.Min.X; x < bounds.Max.X; x += stride {
			off := rowOff + (x-bounds.Min.X)*4
			a := pix[off+3]
			if a == 0 {
				continue
			}
			r, g, b := pix[off], pix[off+1], pix[off+2]
			key := uint32(r)<<16 | uint32(g)<<8 | uint32(b)
			colorSet[key] = struct{}{}
		}
	}
}

func sampleRGBA(src *image.RGBA, bounds image.Rectangle, stride int, colorSet map[uint32]struct{}) {
	pix := src.Pix
	imgStride := src.Stride
	for y := bounds.Min.Y; y < bounds.Max.Y; y += stride {
		rowOff := (y-bounds.Min.Y)*imgStride + (0)*4
		for x := bounds.Min.X; x < bounds.Max.X; x += stride {
			off := rowOff + (x-bounds.Min.X)*4
			a := pix[off+3]
			if a == 0 {
				continue
			}
			// Pre-multiplied alpha → straight: r = r*255/a
			if a == 255 {
				key := uint32(pix[off])<<16 | uint32(pix[off+1])<<8 | uint32(pix[off+2])
				colorSet[key] = struct{}{}
			} else {
				r := uint8(uint16(pix[off]) * 255 / uint16(a))
				g := uint8(uint16(pix[off+1]) * 255 / uint16(a))
				b := uint8(uint16(pix[off+2]) * 255 / uint16(a))
				key := uint32(r)<<16 | uint32(g)<<8 | uint32(b)
				colorSet[key] = struct{}{}
			}
		}
	}
}

func samplePaletted(src *image.Paletted, bounds image.Rectangle, stride int, colorSet map[uint32]struct{}) {
	for y := bounds.Min.Y; y < bounds.Max.Y; y += stride {
		for x := bounds.Min.X; x < bounds.Max.X; x += stride {
			idx := src.ColorIndexAt(x, y)
			c := src.Palette[idx]
			r, g, b, a := c.RGBA()
			if a == 0 {
				continue
			}
			key := uint32(uint8(r>>8))<<16 | uint32(uint8(g>>8))<<8 | uint32(uint8(b>>8))
			colorSet[key] = struct{}{}
		}
	}
}

func sampleGeneric(img image.Image, bounds image.Rectangle, stride int, colorSet map[uint32]struct{}) {
	for y := bounds.Min.Y; y < bounds.Max.Y; y += stride {
		for x := bounds.Min.X; x < bounds.Max.X; x += stride {
			c := img.At(x, y)
			r, g, b, a := c.RGBA()
			if a == 0 {
				continue
			}
			key := uint32(uint8(r>>8))<<16 | uint32(uint8(g>>8))<<8 | uint32(uint8(b>>8))
			colorSet[key] = struct{}{}
		}
	}
}
