package materialyou

import (
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

func matmul(row [3]float64, mat *[3][3]float64) [3]float64 {
	r0 := row[0]
	r1 := row[1]
	r2 := row[2]
	return [3]float64{
		r0*mat[0][0] + r1*mat[0][1] + r2*mat[0][2],
		r0*mat[1][0] + r1*mat[1][1] + r2*mat[1][2],
		r0*mat[2][0] + r1*mat[2][1] + r2*mat[2][2],
	}
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

func argbFromLinrgb(linrgb [3]float64) RGB {
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

type ViewingConditions struct {
	N      float64
	Aw     float64
	Nbb    float64
	Ncb    float64
	C      float64
	Nc     float64
	RgbD   [3]float64
	Fl     float64
	FLRoot float64
	Z      float64
}

func makeVc(whitePoint [3]float64, adaptingLuminance float64, backgroundLstar float64, surround float64) ViewingConditions {
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
	rgbD := [3]float64{
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
	rgbAF := [3]float64{fac(0, rW), fac(1, gW), fac(2, bW)}

	var rgbA [3]float64
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

var d65 = [3]float64{95.047, 100.0, 108.883}
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
		af := math.Pow(vc.Fl*math.Abs(comp)/100.0, 0.42)
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

var hexDigits = [16]byte{'0', '1', '2', '3', '4', '5', '6', '7', '8', '9', 'a', 'b', 'c', 'd', 'e', 'f'}

func rgbToHex(c RGB) string {
	r := byte(clampInt(0, 255, int(math.Floor(c.R+0.5))))
	g := byte(clampInt(0, 255, int(math.Floor(c.G+0.5))))
	b := byte(clampInt(0, 255, int(math.Floor(c.B+0.5))))
	var buf [7]byte
	buf[0] = '#'
	buf[1] = hexDigits[r>>4]
	buf[2] = hexDigits[r&0x0f]
	buf[3] = hexDigits[g>>4]
	buf[4] = hexDigits[g&0x0f]
	buf[5] = hexDigits[b>>4]
	buf[6] = hexDigits[b&0x0f]
	return string(buf[:])
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

func hexFromHct(hct HCT) string {
	return rgbToHex(solveToRgb(hct.Hue, hct.Chroma, hct.Tone))
}

func solveToRgb(hueDegrees, chroma, lstar float64) RGB {
	if chroma < 0.0001 || lstar < 0.0001 || lstar > 99.9999 {
		return argbFromLstar(lstar)
	}
	hue := sanitizeDegrees(hueDegrees)
	hueRadians := hue / 180.0 * math.Pi
	y := yFromLstar(lstar)

	exact, ok := findResultByJ(hueRadians, chroma, y)
	if ok {
		return argbFromLinrgb(exact)
	}
	return argbFromLinrgb(bisectToLimit(y, hueRadians))
}

func inverseChromaticAdaptation(adapted float64) float64 {
	aAbs := math.Abs(adapted)
	base := max0(27.13 * aAbs / (400.0 - aAbs))
	return signum(adapted) * math.Pow(base, 1.0/0.42)
}

func findResultByJ(hueRadians, chroma, y float64) ([3]float64, bool) {
	tInnerCoeff := 1.0 / math.Pow(1.64-math.Pow(0.29, vc.N), 0.73)
	eHue := 0.25 * (math.Cos(hueRadians+2.0) + 3.8)
	p1 := eHue * (50000.0 / 13.0) * vc.Nc * vc.Ncb
	hSin := math.Sin(hueRadians)
	hCos := math.Cos(hueRadians)

	j := math.Sqrt(y) * 11.0
	for round := 0; round < 5; round++ {
		jNorm := j / 100.0
		alpha := 0.0
		if chroma != 0.0 && j != 0.0 {
			alpha = chroma / math.Sqrt(jNorm)
		}
		t := math.Pow(alpha*tInnerCoeff, 1.0/0.9)
		ac := vc.Aw * math.Pow(jNorm, 1.0/vc.C/vc.Z)
		p2 := ac / vc.Nbb
		gamma := 23.0 * (p2 + 0.305) * t / (23.0*p1 + 11.0*t*hCos + 108.0*t*hSin)
		a := gamma * hCos
		b := gamma * hSin
		rA := (460.0*p2 + 451.0*a + 288.0*b) / 1403.0
		gA := (460.0*p2 - 891.0*a - 261.0*b) / 1403.0
		bA := (460.0*p2 - 220.0*a - 6300.0*b) / 1403.0

		linrgb := matmul([3]float64{
			inverseChromaticAdaptation(rA),
			inverseChromaticAdaptation(gA),
			inverseChromaticAdaptation(bA),
		}, &linrgbFromScaledDiscount)

		l0 := linrgb[0]
		l1 := linrgb[1]
		l2 := linrgb[2]

		if l0 < 0.0 || l1 < 0.0 || l2 < 0.0 {
			return [3]float64{}, false
		}
		fnj := 0.2126*l0 + 0.7152*l1 + 0.0722*l2
		if fnj <= 0.0 {
			return [3]float64{}, false
		}
		if round == 4 || math.Abs(fnj-y) < 0.002 {
			if l0 > 100.01 || l1 > 100.01 || l2 > 100.01 {
				return [3]float64{}, false
			}
			return linrgb, true
		}
		j -= (fnj - y) * j / (2.0 * fnj)
	}
	return [3]float64{}, false
}

type stepState struct {
	left        [3]float64
	right       [3]float64
	leftHue     float64
	rightHue    float64
	initialized bool
	uncut       bool
}

func bisectToSegment(y, targetHue float64) [2][3]float64 {
	st := stepState{
		left:  [3]float64{-1.0, -1.0, -1.0},
		right: [3]float64{-1.0, -1.0, -1.0},
	}
	for n := 0; n < 12; n++ {
		mid, ok := nthVertex(y, n)
		if !ok {
			continue
		}
		midHue := hueOf(mid)
		if !st.initialized {
			st = stepState{
				left:        mid,
				right:       mid,
				leftHue:     midHue,
				rightHue:    midHue,
				initialized: true,
				uncut:       true,
			}
		} else if st.uncut || areInCyclicOrder(st.leftHue, midHue, st.rightHue) {
			if areInCyclicOrder(st.leftHue, targetHue, midHue) {
				st.uncut = false
				st.right = mid
				st.rightHue = midHue
			} else {
				st.uncut = false
				st.left = mid
				st.leftHue = midHue
			}
		}
	}
	return [2][3]float64{st.left, st.right}
}

func midpoint(a, b [3]float64) [3]float64 {
	return [3]float64{(a[0] + b[0]) / 2.0, (a[1] + b[1]) / 2.0, (a[2] + b[2]) / 2.0}
}

func criticalPlaneBelow(x float64) int {
	return int(math.Floor(x - 0.5))
}

func criticalPlaneAbove(x float64) int {
	return int(math.Ceil(x - 0.5))
}

type planeState struct {
	left    [3]float64
	right   [3]float64
	leftHue float64
	lPlane  int
	rPlane  int
}

func bisectToLimit(y, targetHue float64) [3]float64 {
	seg := bisectToSegment(y, targetHue)
	left := seg[0]
	right := seg[1]

	for axis := 0; axis < 3; axis++ {
		if left[axis] == right[axis] {
			continue
		}
		leftBelow := left[axis] < right[axis]
		var lPlane0, rPlane0 int
		if leftBelow {
			lPlane0 = criticalPlaneBelow(trueDelinearized(left[axis]))
			rPlane0 = criticalPlaneAbove(trueDelinearized(right[axis]))
		} else {
			lPlane0 = criticalPlaneAbove(trueDelinearized(left[axis]))
			rPlane0 = criticalPlaneBelow(trueDelinearized(right[axis]))
		}

		s := planeState{
			left:    left,
			right:   right,
			leftHue: hueOf(left),
			lPlane:  lPlane0,
			rPlane:  rPlane0,
		}

		for i := 0; i < 8; i++ {
			if math.Abs(float64(s.rPlane-s.lPlane)) <= 1.0 {
				break
			}
			mPlane := (s.lPlane + s.rPlane) / 2
			if mPlane < 0 || mPlane >= len(criticalPlanes) {
				break
			}
			coord := criticalPlanes[mPlane]
			mid := setCoordinate(s.left, coord, s.right, axis)
			midHue := hueOf(mid)

			if areInCyclicOrder(s.leftHue, targetHue, midHue) {
				s.right = mid
				s.rPlane = mPlane
			} else {
				s.left = mid
				s.leftHue = midHue
				s.lPlane = mPlane
			}
		}
		left = s.left
		right = s.right
	}
	return midpoint(left, right)
}

func chromaticAdaptation(comp float64) float64 {
	af := math.Pow(math.Abs(comp), 0.42)
	return signum(comp) * 400.0 * af / (af + 27.13)
}

func hueOf(linrgb [3]float64) float64 {
	sd := matmul(linrgb, &scaledDiscountFromLinrgb)
	rA := chromaticAdaptation(sd[0])
	gA := chromaticAdaptation(sd[1])
	bA := chromaticAdaptation(sd[2])
	a := (11.0*rA - 12.0*gA + bA) / 11.0
	b := (rA + gA - 2.0*bA) / 9.0
	return math.Atan2(b, a)
}

func areInCyclicOrder(a, b, c float64) bool {
	return sanitizeRadians(b-a) < sanitizeRadians(c-a)
}

func intercept(source, mid, target float64) float64 {
	return (mid - source) / (target - source)
}

func lerpPoint(source [3]float64, t float64, target [3]float64) [3]float64 {
	return [3]float64{
		source[0] + (target[0]-source[0])*t,
		source[1] + (target[1]-source[1])*t,
		source[2] + (target[2]-source[2])*t,
	}
}

func setCoordinate(source [3]float64, coord float64, target [3]float64, axis int) [3]float64 {
	return lerpPoint(source, intercept(source[axis], coord, target[axis]), target)
}

func isBounded(x float64) bool {
	return 0.0 <= x && x <= 100.0
}

func nthVertex(y float64, n int) ([3]float64, bool) {
	kR := yFromLinrgb[0]
	kG := yFromLinrgb[1]
	kB := yFromLinrgb[2]

	coordA := 0.0
	if n%4 > 1 {
		coordA = 100.0
	}
	coordB := 0.0
	if n%2 != 0 {
		coordB = 100.0
	}

	if n < 4 {
		g := coordA
		b := coordB
		r := (y - g*kG - b*kB) / kR
		if isBounded(r) {
			return [3]float64{r, g, b}, true
		}
		return [3]float64{}, false
	} else if n < 8 {
		b := coordA
		r := coordB
		g := (y - r*kR - b*kB) / kG
		if isBounded(g) {
			return [3]float64{r, g, b}, true
		}
		return [3]float64{}, false
	} else {
		r := coordA
		g := coordB
		b := (y - r*kR - g*kG) / kB
		if isBounded(b) {
			return [3]float64{r, g, b}, true
		}
		return [3]float64{}, false
	}
}

var scaledDiscountFromLinrgb = [3][3]float64{
	{0.001200833568784504, 0.002389694492170889, 0.0002795742885861124},
	{0.0005891086651375999, 0.0029785502573438758, 0.0003270666104008398},
	{0.00010146692491640572, 0.0005364214359186694, 0.0032979401770712076},
}

var linrgbFromScaledDiscount = [3][3]float64{
	{1373.2198709594231, -1100.4251190754821, -7.278681089101213},
	{-271.815969077903, 559.6580465940733, -32.46047482791194},
	{1.9622899599665666, -57.173814538844006, 308.7233197812385},
}

var yFromLinrgb = [3]float64{0.2126, 0.7152, 0.0722}
