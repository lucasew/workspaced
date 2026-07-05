package materialyou

import (
	"fmt"
	"math"
	"strconv"
	"strings"
)

func max0(x float64) float64 {
	if x > 0.0 {
		return x
	}
	return 0.0
}

func matmul(row []float64, mat [][]float64) []float64 {
	r0 := row[0]
	r1 := row[1]
	r2 := row[2]
	dot := func(v []float64) float64 {
		return r0*v[0] + r1*v[1] + r2*v[2]
	}
	return []float64{dot(mat[0]), dot(mat[1]), dot(mat[2])}
}

var labE = 216.0 / 24389.0
var labKappa = 24389.0 / 27.0

func labF(t float64) float64 {
	if t > labE {
		return math.Cbrt(t)
	}
	return (labKappa*t + 16.0) / 116.0
}

func labInvf(ft float64) float64 {
	ft3 := ft * ft * ft
	if ft3 > labE {
		return ft3
	}
	return (116.0*ft - 16.0) / labKappa
}

func yFromLstar(l float64) float64 {
	return 100.0 * labInvf((l+16.0)/116.0)
}

func lstarFromY(y float64) float64 {
	return labF(y/100.0)*116.0 - 16.0
}

func linearized(c float64) float64 {
	n := c / 255.0
	if n <= 0.040449936 {
		return n / 12.92 * 100.0
	}
	return math.Pow((n+0.055)/1.055, 2.4) * 100.0
}

func delinearizedRaw(v float64) float64 {
	n := v / 100.0
	if n <= 0.0031308 {
		return n * 12.92
	}
	return 1.055*math.Pow(n, 1.0/2.4) - 0.055
}

func delinearized(v float64) float64 {
	return float64(clampInt(0, 255, int(round(delinearizedRaw(v)*255.0))))
}

func trueDelinearized(v float64) float64 {
	return delinearizedRaw(v) * 255.0
}

type RGB struct {
	R float64
	G float64
	B float64
}

func argbFromLinrgb(linrgb []float64) RGB {
	return RGB{
		R: delinearized(linrgb[0]),
		G: delinearized(linrgb[1]),
		B: delinearized(linrgb[2]),
	}
}

func argbFromLstar(l float64) RGB {
	c := delinearized(yFromLstar(l))
	return RGB{R: c, G: c, B: c}
}

func lstarFromRgb(c RGB) float64 {
	y := 0.2126*linearized(c.R) + 0.7152*linearized(c.G) + 0.0722*linearized(c.B)
	return 116.0*labF(y/100.0) - 16.0
}

var d65 = []float64{95.047, 100.0, 108.883}

type ViewingConditions struct {
	N      float64
	Aw     float64
	Nbb    float64
	Ncb    float64
	C      float64
	Nc     float64
	RgbD   []float64
	Fl     float64
	FLRoot float64
	Z      float64
}

func makeVc(whitePoint []float64, adaptingLuminance float64, backgroundLstar float64, surround float64) ViewingConditions {
	rW := whitePoint[0]*0.401288 + whitePoint[1]*0.650173 + whitePoint[2]*(-0.051461)
	gW := whitePoint[0]*(-0.250268) + whitePoint[1]*1.204414 + whitePoint[2]*0.045854
	bW := whitePoint[0]*(-0.002079) + whitePoint[1]*0.048952 + whitePoint[2]*0.953127

	f := 0.8 + surround/10.0
	lerp := func(a, b, t float64) float64 { return (1.0-t)*a + t*b }

	c := 0.0
	if f >= 0.9 {
		c = lerp(0.59, 0.69, (f-0.9)*10.0)
	} else {
		c = lerp(0.525, 0.59, (f-0.8)*10.0)
	}

	dRaw := f * (1.0 - (1.0/3.6)*math.Exp((0.0-adaptingLuminance-42.0)/92.0))
	d := dRaw
	if d > 1.0 {
		d = 1.0
	} else if d < 0.0 {
		d = 0.0
	}

	nc := f
	rgbD := []float64{
		d*(100.0/rW) + 1.0 - d,
		d*(100.0/gW) + 1.0 - d,
		d*(100.0/bW) + 1.0 - d,
	}

	k := 1.0 / (5.0*adaptingLuminance + 1.0)
	k4 := k * k * k * k
	k4F := 1.0 - k4
	fl := k4*adaptingLuminance + 0.1*k4F*k4F*math.Cbrt(5.0*adaptingLuminance)

	n := yFromLstar(backgroundLstar) / whitePoint[1]
	z := 1.48 + math.Sqrt(n)
	nbb := 0.725 / math.Pow(n, 0.2)

	fac := func(i int, rw float64) float64 { return math.Pow(fl*rgbD[i]*rw/100.0, 0.42) }
	rgbAF := []float64{fac(0, rW), fac(1, gW), fac(2, bW)}

	rgbA := make([]float64, 3)
	for i, x := range rgbAF {
		rgbA[i] = 400.0 * x / (x + 27.13)
	}

	aw := (2.0*rgbA[0] + rgbA[1] + 0.05*rgbA[2]) * nbb

	return ViewingConditions{
		N:      n,
		Aw:     aw,
		Nbb:    nbb,
		Ncb:    nbb,
		C:      c,
		Nc:     nc,
		RgbD:   rgbD,
		Fl:     fl,
		FLRoot: math.Pow(fl, 0.25),
		Z:      z,
	}
}

var vc = makeVc(d65, (200.0/math.Pi)*yFromLstar(50.0)/100.0, 50.0, 2.0)

type CAM16 struct {
	Hue    float64
	Chroma float64
}

func cam16(c RGB) CAM16 {
	rL := linearized(c.R)
	gL := linearized(c.G)
	bL := linearized(c.B)
	x := 0.41233895*rL + 0.35762064*gL + 0.18051042*bL
	y := 0.2126*rL + 0.7152*gL + 0.0722*bL
	z := 0.01932141*rL + 0.11916382*gL + 0.95034478*bL

	rC := 0.401288*x + 0.650173*y - 0.051461*z
	gC := (-0.250268)*x + 1.204414*y + 0.045854*z
	bC := (-0.002079)*x + 0.048952*y + 0.953127*z

	rD := vc.RgbD[0] * rC
	gD := vc.RgbD[1] * gC
	bD := vc.RgbD[2] * bC

	adapt := func(comp float64) float64 {
		af := math.Pow(vc.Fl*abs(comp)/100.0, 0.42)
		return signum(comp) * 400.0 * af / (af + 27.13)
	}

	rA := adapt(rD)
	gA := adapt(gD)
	bA := adapt(bD)

	a := (11.0*rA - 12.0*gA + bA) / 11.0
	bb := (rA + gA - 2.0*bA) / 9.0
	u := (20.0*rA + 20.0*gA + 21.0*bA) / 20.0
	p2 := (40.0*rA + 20.0*gA + bA) / 20.0

	hue := sanitizeDegrees(math.Atan2(bb, a) * 180.0 / math.Pi)
	ac := p2 * vc.Nbb
	j := 100.0 * math.Pow(ac/vc.Aw, vc.C*vc.Z)

	huePrime := hue
	if hue < 20.14 {
		huePrime += 360.0
	}
	eHue := 0.25 * (math.Cos(huePrime*math.Pi/180.0+2.0) + 3.8)
	p1 := (50000.0 / 13.0) * eHue * vc.Nc * vc.Ncb
	t := p1 * math.Sqrt(a*a+bb*bb) / (u + 0.305)
	alpha := math.Pow(t, 0.9) * math.Pow(1.64-math.Pow(0.29, vc.N), 0.73)
	chroma := alpha * math.Sqrt(j/100.0)

	return CAM16{Hue: hue, Chroma: chroma}
}

func hexToRgb(hex string) RGB {
	hex = strings.TrimPrefix(hex, "#")
	if len(hex) != 6 {
		return RGB{}
	}
	r, _ := strconv.ParseInt(hex[0:2], 16, 64)
	g, _ := strconv.ParseInt(hex[2:4], 16, 64)
	b, _ := strconv.ParseInt(hex[4:6], 16, 64)
	return RGB{R: float64(r), G: float64(g), B: float64(b)}
}

func rgbToHex(c RGB) string {
	r := clampInt(0, 255, int(math.Floor(c.R+0.5)))
	g := clampInt(0, 255, int(math.Floor(c.G+0.5)))
	b := clampInt(0, 255, int(math.Floor(c.B+0.5)))
	return fmt.Sprintf("#%02x%02x%02x", r, g, b)
}

type HCT struct {
	Hue    float64
	Chroma float64
	Tone   float64
}

func hctFromHex(hex string) HCT {
	rgb := hexToRgb(hex)
	c := cam16(rgb)
	return HCT{
		Hue:    c.Hue,
		Chroma: c.Chroma,
		Tone:   lstarFromRgb(rgb),
	}
}
