package genetic

import (
	"math/rand"

	"workspaced/pkg/palette/api"
)

const (
	numSurvivors = 500
	numNewborns  = 49500
	mutationRate = 0.75
)

// initPopulation creates the initial random population with random colors.
// Evolution converges toward image colors through the fitness function.
func initPopulation(rng *rand.Rand, count int, size int) []Individual {
	backing := make([]api.LAB, size*count)
	population := make([]Individual, size)
	for i := range size {
		colors := backing[i*count : (i+1)*count : (i+1)*count]
		for j := range count {
			colors[j] = api.LAB{
				L: rng.Float64() * 100,     // Lightness: 0-100
				A: rng.Float64()*200 - 100, // Green-Red: -100 to +100
				B: rng.Float64()*200 - 100, // Blue-Yellow: -100 to +100
			}
		}
		population[i] = Individual{colors: colors}
	}
	return population
}

// crossoverInto combines two parents using alternating zip (Stylix Ai/Evolutionary.hs).
// Writes result into dst to avoid per-call allocation.
func crossoverInto(p1, p2 Individual, dst []api.LAB) {
	size := min(len(p2.colors), len(p1.colors))
	for i := range size {
		if i%2 == 0 {
			dst[i] = p1.colors[i]
		} else {
			dst[i] = p2.colors[i]
		}
	}
}

// mutateInPlace replaces one color from imageColors with probability rate.
// Operates on colors in-place (caller owns the slice).
func mutateInPlace(rng *rand.Rand, colors []api.LAB, imageColors []api.LAB, rate float64) {
	if len(imageColors) == 0 || rng.Float64() > rate {
		return
	}
	pos := rng.Intn(len(colors))
	colors[pos] = imageColors[rng.Intn(len(imageColors))]
}

// evolve creates the next generation from survivors (Stylix Ai/Evolutionary.hs).
func evolve(rng *rand.Rand, survivors []scoredIndividual, imageColors []api.LAB) []Individual {
	total := numSurvivors + numNewborns
	count := len(survivors[0].individual.colors)
	backing := make([]api.LAB, total*count)
	newPopulation := make([]Individual, total)

	// Copy the best survivor into the new backing.
	if len(survivors) > 0 {
		copy(backing[0:count], survivors[0].individual.colors)
		newPopulation[0] = Individual{colors: backing[0:count:count]}
	}
	for i := 1; i < total; i++ {
		dst := backing[i*count : (i+1)*count : (i+1)*count]
		p1 := survivors[rng.Intn(len(survivors))].individual
		p2 := survivors[rng.Intn(len(survivors))].individual
		crossoverInto(p1, p2, dst)
		mutateInPlace(rng, dst, imageColors, mutationRate)
		newPopulation[i] = Individual{colors: dst}
	}
	return newPopulation
}
