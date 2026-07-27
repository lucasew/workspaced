package driver

// Clamp01 clamps v to [0, 1].
func Clamp01(v float64) float64 {
	if v > 1 {
		return 1
	}
	if v < 0 {
		return 0
	}
	return v
}
