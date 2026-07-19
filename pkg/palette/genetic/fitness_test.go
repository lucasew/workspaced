package genetic

import (
	"math"
	"math/rand"
	"testing"

	"github.com/lucasew/workspaced/pkg/palette/api"
)

func TestAccentPairStatsMatchesHistoricalTwoPass(t *testing.T) {
	t.Parallel()
	accents := []api.LAB{
		{L: 50, A: 40, B: 20},
		{L: 50, A: 41, B: 20}, // near-duplicate of first
		{L: 60, A: -30, B: 40},
		{L: 40, A: 10, B: -50},
		{L: 55, A: 0, B: 0},
		{L: 70, A: 80, B: -10},
		{L: 30, A: -60, B: 30},
		{L: 45, A: 20, B: 70},
	}

	// Historical two-pass reference (must stay inlined so we do not call the fused path).
	minDist := math.MaxFloat64
	var penalty float64
	for i := range accents {
		for j := i + 1; j < len(accents); j++ {
			diff := api.DeltaE(accents[i], accents[j])
			if diff < minDist {
				minDist = diff
			}
			switch {
			case diff < 5.0:
				penalty += 10.0
			case diff < 10.0:
				penalty += 5.0
			case diff < 15.0:
				penalty += 2.0
			}
		}
	}

	gotMin, gotPen := accentPairStats(accents)
	if gotMin != minDist {
		t.Fatalf("minDist: got %v want %v", gotMin, minDist)
	}
	if gotPen != penalty {
		t.Fatalf("penalty: got %v want %v", gotPen, penalty)
	}
}

func TestImageSimilarityMatchesDeltaEScan(t *testing.T) {
	t.Parallel()
	palette := []api.LAB{
		{L: 10, A: 0, B: 0},
		{L: 50, A: 20, B: -10},
		{L: 90, A: -5, B: 15},
	}
	image := []api.LAB{
		{L: 12, A: 1, B: -1},
		{L: 48, A: 18, B: -8},
		{L: 88, A: -4, B: 14},
		{L: 30, A: 40, B: 40},
		{L: 70, A: -40, B: -20},
	}

	// Reference: full DeltaE min scan (historical).
	total := 0.0
	for _, p := range palette {
		minDist := math.MaxFloat64
		for _, img := range image {
			dist := api.DeltaE(p, img)
			if dist < minDist {
				minDist = dist
			}
		}
		total += minDist
	}
	want := -total / float64(len(palette))
	got := calculateImageSimilarity(palette, image)
	if got != want {
		t.Fatalf("image similarity: got %v want %v", got, want)
	}
}

// historicalImageSimilarity is the pre-optimization full-DeltaE scan, kept only
// for microbench comparison (must not be used by production fitness).
func historicalImageSimilarity(paletteColors []api.LAB, imageColors []api.LAB) float64 {
	if len(imageColors) == 0 {
		return 0
	}
	totalDist := 0.0
	for _, palColor := range paletteColors {
		minDist := math.MaxFloat64
		for _, imgColor := range imageColors {
			dist := api.DeltaE(palColor, imgColor)
			if dist < minDist {
				minDist = dist
			}
		}
		totalDist += minDist
	}
	return -totalDist / float64(len(paletteColors))
}

func BenchmarkImageSimilarityOldVsNew(b *testing.B) {
	palette := make([]api.LAB, 16)
	for i := range palette {
		palette[i] = api.LAB{L: float64(i * 6), A: float64(i*7 - 50), B: float64(i*3 - 20)}
	}
	image := make([]api.LAB, 2000)
	for i := range image {
		image[i] = api.LAB{L: float64(i%100) + 0.1, A: float64(i%40) - 20, B: float64(i%30) - 15}
	}
	b.Run("historical_deltae", func(b *testing.B) {
		b.ReportAllocs()
		for b.Loop() {
			_ = historicalImageSimilarity(palette, image)
		}
	})
	b.Run("squared_then_sqrt", func(b *testing.B) {
		b.ReportAllocs()
		for b.Loop() {
			_ = calculateImageSimilarity(palette, image)
		}
	})
}

func BenchmarkCalculateFitness(b *testing.B) {
	rngColors := make([]api.LAB, 16)
	for i := range rngColors {
		rngColors[i] = api.LAB{
			L: float64(i) * 6,
			A: float64(i*7 - 50),
			B: float64(i*3 - 20),
		}
	}
	image := make([]api.LAB, 2000)
	for i := range image {
		image[i] = api.LAB{
			L: float64(i%100) + 0.1,
			A: float64(i%40) - 20,
			B: float64(i%30) - 15,
		}
	}
	ind := Individual{colors: rngColors}
	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		_ = calculateFitness(ind, image, api.PolarityDark)
	}
}

func BenchmarkScorePopSubset(b *testing.B) {
	const n = 200
	pop := initPopulation(rand.New(rand.NewSource(1)), 16, n)
	image := make([]api.LAB, 500)
	for i := range image {
		image[i] = api.LAB{L: float64(i % 100), A: float64(i%50) - 25, B: float64(i%40) - 20}
	}
	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		_ = scorePop(pop, image, api.PolarityDark)
	}
}
