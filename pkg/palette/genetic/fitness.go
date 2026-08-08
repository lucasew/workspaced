package genetic

import (
	"cmp"
	"math"
	"slices"

	"workspaced/pkg/palette/api"
)

// Individual represents a candidate palette solution.
type Individual struct {
	colors []api.LAB
}

// calculateFitness evaluates how good a palette is.
// Based on Stylix Stylix/Palette.hs fitness function.
func calculateFitness(ind Individual, imageColors []api.LAB, polarity api.Polarity) float64 {
	var score float64

	// Primary scale similarity (base00-07 should be similar to each other)
	primarySim := maxDeltaE(ind.colors[:8])
	score -= primarySim / 10.0 // Penalize high maximum distance (want low spread)

	// Accent pairwise stats in one pass: min distance + duplicate penalty.
	// Previously minDeltaE and calculateDuplicatePenalty each scanned C(n,2).
	accentDiff, duplicatePenalty := accentPairStats(ind.colors[8:16])
	score += accentDiff * 2.0        // Double weight - want distinct colors
	score -= duplicatePenalty * 50.0 // Massive penalty for duplicates

	// Penalize accents that are too similar (under threshold)
	if accentDiff < 20.0 {
		score -= (20.0 - accentDiff) * 3.0 // Heavy penalty for similar accents
	}

	// Image similarity (colors should appear in the image)
	imageSim := calculateImageSimilarity(ind.colors, imageColors)
	score += imageSim * 5.0 // Reduced weight to allow more variety

	// Contrast requirements (critical for readability)
	contrastScore := calculateContrastScore(ind.colors)
	score += contrastScore * 5.0 // Strong weight on readability

	// Color diversity (hue variation in accents)
	hueDiversity := calculateHueDiversity(ind.colors[8:16])
	score += hueDiversity * 3.0 // Reward varied hues

	// Lightness scheme matching
	if polarity != api.PolarityAny {
		lightnessError := calculateLightnessError(ind.colors, polarity)
		score -= lightnessError
	}

	return score
}

// deltaESq is squared CIE76 distance. Monotonic with DeltaE, so min/max
// selection over pairs is identical; only the final chosen pair needs Sqrt.
func deltaESq(c1, c2 api.LAB) float64 {
	dl := c1.L - c2.L
	da := c1.A - c2.A
	db := c1.B - c2.B
	return dl*dl + da*da + db*db
}

// maxDeltaE finds the maximum perceptual distance between any two colors.
// Uses squared distance for comparison (monotonic with DeltaE), sqrt only once.
func maxDeltaE(colors []api.LAB) float64 {
	if len(colors) < 2 {
		return 0
	}

	maxSq := 0.0
	for i := range colors {
		for j := i + 1; j < len(colors); j++ {
			if sq := deltaESq(colors[i], colors[j]); sq > maxSq {
				maxSq = sq
			}
		}
	}
	return math.Sqrt(maxSq)
}

// minDeltaE finds the minimum perceptual distance between any two colors.
func minDeltaE(colors []api.LAB) float64 {
	d, _ := accentPairStats(colors)
	return d
}

// accentPairStats computes min pairwise DeltaE and the near-duplicate penalty
// in a single O(n²) pass (same results as separate minDeltaE + penalty scans).
// Uses squared distances internally; thresholds are pre-squared (5²=25, 10²=100, 15²=225).
func accentPairStats(colors []api.LAB) (minDist, duplicatePenalty float64) {
	if len(colors) < 2 {
		return 0, 0
	}

	minSq := math.MaxFloat64
	for i := range colors {
		for j := i + 1; j < len(colors); j++ {
			sq := deltaESq(colors[i], colors[j])
			if sq < minSq {
				minSq = sq
			}
			switch {
			case sq < 25.0: // 5.0²
				duplicatePenalty += 10.0 // Very high penalty per duplicate
			case sq < 100.0: // 10.0²
				duplicatePenalty += 5.0 // High penalty for very similar
			case sq < 225.0: // 15.0²
				duplicatePenalty += 2.0 // Moderate penalty for similar
			}
		}
	}
	return math.Sqrt(minSq), duplicatePenalty
}

// calculateImageSimilarity measures how well the palette matches the image colors.
func calculateImageSimilarity(paletteColors []api.LAB, imageColors []api.LAB) float64 {
	if len(imageColors) == 0 {
		return 0
	}

	// For each palette color, find closest image color.
	// Compare on squared distance (monotonic); one Sqrt for the winner.
	// Same min pair and same distance value as scanning with DeltaE.
	totalDist := 0.0
	for _, palColor := range paletteColors {
		minSq := math.MaxFloat64
		for _, imgColor := range imageColors {
			if sq := deltaESq(palColor, imgColor); sq < minSq {
				minSq = sq
			}
		}
		totalDist += math.Sqrt(minSq)
	}

	// Average distance (lower is better, so negate for score)
	avgDist := totalDist / float64(len(paletteColors))
	return -avgDist
}

// Target lightness patterns (static, never mutated).
var darkLightnesses = [8]float64{10, 30, 45, 65, 75, 90, 95, 95}
var lightLightnesses = [8]float64{90, 70, 55, 35, 25, 10, 5, 5}

// calculateLightnessError measures deviation from target lightness pattern.
// Based on Stylix Stylix/Palette.hs lines 82-94.
func calculateLightnessError(colors []api.LAB, polarity api.Polarity) float64 {
	var targetLightnesses *[8]float64

	switch polarity {
	case api.PolarityDark:
		targetLightnesses = &darkLightnesses
	case api.PolarityLight:
		targetLightnesses = &lightLightnesses
	case api.PolarityAny:
		return 0
	}

	// Calculate error for base00-07 (primary scale)
	errorSum := 0.0
	n := min(8, len(colors))
	for i := 0; i < n; i++ {
		diff := colors[i].L - targetLightnesses[i]
		errorSum += math.Abs(diff)
	}

	return errorSum
}

// calculateContrastScore rewards palettes with good contrast for readability.
func calculateContrastScore(colors []api.LAB) float64 {
	if len(colors) < 16 {
		return 0
	}

	var score float64

	// Base00 (background) vs Base05 (foreground) - must have high contrast
	bgFgContrast := math.Abs(colors[0].L - colors[5].L)
	switch {
	case bgFgContrast >= 50:
		score += 10.0 // Excellent contrast
	case bgFgContrast >= 40:
		score += 5.0 // Good contrast
	default:
		score -= (50 - bgFgContrast) // Penalize poor contrast
	}

	// Base07 (light background) vs Base02 (selection) - moderate contrast
	lightBgSelectionContrast := math.Abs(colors[7].L - colors[2].L)
	if lightBgSelectionContrast >= 15 && lightBgSelectionContrast <= 40 {
		score += 3.0 // Good selection visibility
	}

	// Accent colors (Base08-0F) vs background (Base00) - should be visible
	minAccentContrast := 100.0
	for i := 8; i < 16; i++ {
		contrast := math.Abs(colors[i].L - colors[0].L)
		if contrast < minAccentContrast {
			minAccentContrast = contrast
		}
	}
	if minAccentContrast >= 25 {
		score += 5.0 // All accents visible on background
	} else {
		score -= (25 - minAccentContrast) / 2 // Penalize invisible accents
	}

	return score
}

// calculateDuplicatePenalty heavily penalizes duplicate or near-duplicate colors.
func calculateDuplicatePenalty(colors []api.LAB) float64 {
	_, penalty := accentPairStats(colors)
	return penalty
}

// calculateHueDiversity measures hue variation in accent colors.
// Returns higher score for colors spread across color wheel.
func calculateHueDiversity(colors []api.LAB) float64 {
	if len(colors) < 2 {
		return 0
	}

	// Calculate variance in A and B channels (chromaticity)
	var aSum, bSum float64
	for _, c := range colors {
		aSum += c.A
		bSum += c.B
	}
	aMean := aSum / float64(len(colors))
	bMean := bSum / float64(len(colors))

	var aVariance, bVariance float64
	for _, c := range colors {
		aDiff := c.A - aMean
		bDiff := c.B - bMean
		aVariance += aDiff * aDiff
		bVariance += bDiff * bDiff
	}
	aVariance /= float64(len(colors))
	bVariance /= float64(len(colors))

	// Higher variance = more color diversity
	diversity := math.Sqrt(aVariance + bVariance)

	// Normalize to reasonable range (0-20)
	return math.Min(diversity/5.0, 20.0)
}

// scoredIndividual pairs an individual with its fitness score.
type scoredIndividual struct {
	individual Individual
	fitness    float64
}

// scorePop calculates fitness for entire population and sorts by fitness.
func scorePop(population []Individual, imageColors []api.LAB, polarity api.Polarity) []scoredIndividual {
	scored := make([]scoredIndividual, len(population))

	for i, ind := range population {
		fitness := calculateFitness(ind, imageColors, polarity)
		scored[i] = scoredIndividual{
			individual: ind,
			fitness:    fitness,
		}
	}

	// Stable sort keeps input order on ties so deterministic RNG paths stay fixed.
	slices.SortStableFunc(scored, func(a, b scoredIndividual) int {
		return cmp.Compare(b.fitness, a.fitness)
	})
	return scored
}
