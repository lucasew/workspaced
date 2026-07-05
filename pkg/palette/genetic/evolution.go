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
	population := make([]Individual, size)
	for i := range size {
		individual := Individual{
			colors: make([]api.LAB, count),
		}
		for j := range count {
			individual.colors[j] = api.LAB{
				L: rng.Float64() * 100,     // Lightness: 0-100
				A: rng.Float64()*200 - 100, // Green-Red: -100 to +100
				B: rng.Float64()*200 - 100, // Blue-Yellow: -100 to +100
			}
		}
		population[i] = individual
	}
	return population
}

// crossover combines two parents using alternating zip (Stylix Ai/Evolutionary.hs).
func crossover(p1, p2 Individual) Individual {
	size := min(len(p2.colors), len(p1.colors))
	offspring := Individual{
		colors: make([]api.LAB, size),
	}
	for i := range size {
		if i%2 == 0 {
			offspring.colors[i] = p1.colors[i]
		} else {
			offspring.colors[i] = p2.colors[i]
		}
	}
	return offspring
}

// mutate replaces one color from imageColors with probability rate.
func mutate(rng *rand.Rand, ind Individual, imageColors []api.LAB, rate float64) Individual {
	if len(imageColors) == 0 || rng.Float64() > rate {
		return ind
	}
	mutated := Individual{
		colors: make([]api.LAB, len(ind.colors)),
	}
	copy(mutated.colors, ind.colors)
	pos := rng.Intn(len(mutated.colors))
	mutated.colors[pos] = imageColors[rng.Intn(len(imageColors))]
	return mutated
}

// evolve creates the next generation from survivors (Stylix Ai/Evolutionary.hs).
func evolve(rng *rand.Rand, survivors []scoredIndividual, imageColors []api.LAB) []Individual {
	newPopulation := make([]Individual, 0, numSurvivors+numNewborns)
	if len(survivors) > 0 {
		newPopulation = append(newPopulation, survivors[0].individual)
	}
	for i := 1; i < numSurvivors+numNewborns; i++ {
		p1 := survivors[rng.Intn(len(survivors))].individual
		p2 := survivors[rng.Intn(len(survivors))].individual
		offspring := mutate(rng, crossover(p1, p2), imageColors, mutationRate)
		newPopulation = append(newPopulation, offspring)
	}
	return newPopulation
}
