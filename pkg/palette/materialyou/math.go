package materialyou

import (
	"math"
)

func clampInt(lo, hi, x int) int {
	if x < lo {
		return lo
	}
	if x > hi {
		return hi
	}
	return x
}

func round(x float64) float64 {
	return math.Floor(x + 0.5)
}

func signum(x float64) float64 {
	if x < 0.0 {
		return -1.0
	}
	if x == 0.0 {
		return 0.0
	}
	return 1.0
}

func sanitizeDegrees(d float64) float64 {
	r := math.Mod(d, 360.0)
	if r < 0.0 {
		return r + 360.0
	}
	return r
}

func sanitizeRadians(a float64) float64 {
	return math.Mod(a+math.Pi*8.0, math.Pi*2.0)
}
