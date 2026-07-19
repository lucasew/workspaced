package genetic

import (
	"context"
	"image"
	"math/rand"

	"github.com/lucasew/workspaced/pkg/logging"
	"github.com/lucasew/workspaced/pkg/palette/api"
)

func init() {
	api.Register(&Driver{})
}

type Driver struct{}

func (d *Driver) Name() string {
	return "genetic"
}

func (d *Driver) Description() string {
	return "Stylix-style evolutionary search for harmonious base16/base24 colors"
}

func (d *Driver) Extract(ctx context.Context, img image.Image, opts api.Options) (*api.Palette, error) {
	// Deterministic RNG for reproducibility (like Stylix).
	rng := rand.New(rand.NewSource(42))

	colors := api.SampleImage(img, opts.MaxSamples)
	if len(colors) == 0 {
		return nil, ctx.Err()
	}
	logger := logging.GetLogger(ctx)
	logger.Info("sampled colors from image", "unique_colors", len(colors))

	labColors := make([]api.LAB, len(colors))
	for i, c := range colors {
		labColors[i] = api.RGBToLAB(c)
	}
	logger.Info("converted to LAB color space")

	population := initPopulation(rng, opts.ColorCount, numSurvivors+numNewborns)
	logger.Info("initialized population", "size", len(population), "colors_per_palette", opts.ColorCount)

	generation := 0
	var prevBestFitness float64
	const maxGenerations = 100

	for generation < maxGenerations {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}

		scored := scorePop(population, labColors, opts.Polarity)

		bestFitness := scored[0].fitness
		logger.Info("generation completed",
			"generation", generation,
			"best_fitness", bestFitness,
			"population_size", len(population))

		if generation > 0 && bestFitness == prevBestFitness {
			logger.Info("converged - fitness unchanged", "generations", generation)
			break
		}
		prevBestFitness = bestFitness

		survivors := scored
		if len(survivors) > numSurvivors {
			survivors = survivors[:numSurvivors]
		}

		population = evolve(rng, survivors, labColors)
		generation++
	}

	scored := scorePop(population, labColors, opts.Polarity)
	best := scored[0].individual
	logger.Info("evolution complete", "final_fitness", scored[0].fitness, "total_generations", generation)

	pal := mapToPalette(best, opts.ColorCount)
	logger.Info("palette generated successfully")
	return pal, nil
}

// mapToPalette converts an individual to a base16/base24 palette.
func mapToPalette(ind Individual, colorCount int) *api.Palette {
	hexColors := make([]string, len(ind.colors))
	for i, lab := range ind.colors {
		hexColors[i] = api.ToHex(api.LABToRGB(lab))
	}

	n := 0
	if len(hexColors) >= 16 {
		n = 16
	}
	if colorCount >= 24 && len(hexColors) >= 24 {
		n = 24
	}
	return api.PaletteFromHexes(hexColors[:n])
}
