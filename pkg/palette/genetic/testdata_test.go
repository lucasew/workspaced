package genetic

import (
	"image"
	_ "image/jpeg"
	_ "image/png"
	"math/rand"
	"os"
	"path/filepath"
	"testing"

	"workspaced/pkg/logging"
	"workspaced/pkg/palette/api"
)

func paletteTestdata(t testing.TB, name string) string {
	t.Helper()
	return filepath.Join("..", "testdata", name)
}

func loadPaletteImage(t testing.TB, name string) image.Image {
	t.Helper()
	f, err := os.Open(paletteTestdata(t, name))
	if err != nil {
		t.Fatalf("open testdata %s: %v", name, err)
	}
	defer f.Close()
	img, _, err := image.Decode(f)
	if err != nil {
		t.Fatalf("decode testdata %s: %v", name, err)
	}
	return img
}

func TestGeneticScoreFromBlocksTestdata(t *testing.T) {
	t.Parallel()
	// Cheap match path: sample fixture colors and score a tiny population.
	// Full Extract is opt-in (WORKSPACED_TEST_GENETIC_EXTRACT=1) / benchmarks only.
	img := loadPaletteImage(t, "blocks_64.png")
	colors := api.SampleImage(img, 0)
	if len(colors) != 4 {
		t.Fatalf("blocks_64 unique colors = %d, want 4", len(colors))
	}
	lab := make([]api.LAB, len(colors))
	for i, c := range colors {
		lab[i] = api.RGBToLAB(c)
	}
	pop := initPopulation(rand.New(rand.NewSource(42)), 16, 32)
	scored := scorePop(pop, lab, api.PolarityDark)
	if len(scored) != 32 {
		t.Fatalf("scored len = %d", len(scored))
	}
	if scored[0].fitness < scored[len(scored)-1].fitness {
		t.Fatal("expected scorePop sorted by fitness descending")
	}
	pal := mapToPalette(scored[0].individual, 16)
	if pal.Base00 == "" || pal.Base0F == "" {
		t.Fatalf("incomplete palette: base00=%q base0F=%q", pal.Base00, pal.Base0F)
	}
}

func TestGeneticScoreFromBlissTestdata(t *testing.T) {
	t.Parallel()
	img := loadPaletteImage(t, "bliss.jpg")
	colors := api.SampleImage(img, 10000)
	if len(colors) == 0 {
		t.Fatal("bliss sample empty")
	}
	lab := make([]api.LAB, len(colors))
	for i, c := range colors {
		lab[i] = api.RGBToLAB(c)
	}
	pop := initPopulation(rand.New(rand.NewSource(42)), 16, 32)
	scored := scorePop(pop, lab, api.PolarityDark)
	if scored[0].fitness < scored[len(scored)-1].fitness {
		t.Fatal("expected scorePop sorted by fitness descending")
	}
	pal := mapToPalette(scored[0].individual, 16)
	if pal.Base00 == "" || pal.Base0F == "" {
		t.Fatalf("incomplete palette: base00=%q base0F=%q", pal.Base00, pal.Base0F)
	}
}

func TestGeneticExtractFromTestdata(t *testing.T) {
	t.Parallel()
	if testing.Short() || os.Getenv("WORKSPACED_TEST_GENETIC_EXTRACT") == "" {
		t.Skip("set WORKSPACED_TEST_GENETIC_EXTRACT=1 (and not -short) for full evolution")
	}
	// Prefer real wallpaper when available; MaxSamples matches CLI default.
	img := loadPaletteImage(t, "bliss.jpg")
	ctx := logging.NewRootContext(nil)
	d := &Driver{}
	pal, err := d.Extract(ctx, img, api.Options{
		Polarity:   api.PolarityDark,
		ColorCount: 16,
		MaxSamples: 10000,
	})
	if err != nil {
		t.Fatal(err)
	}
	if pal.Base00 == "" || pal.Base0F == "" {
		t.Fatalf("incomplete palette: base00=%q base0F=%q", pal.Base00, pal.Base0F)
	}
	if len(pal.Base00) != 6 || pal.Base00[0] == '#' {
		t.Fatalf("expected 6-digit hex without '#', got %q", pal.Base00)
	}
}

func BenchmarkGeneticExtract(b *testing.B) {
	if testing.Short() {
		b.Skip("skipping full genetic extract benchmark in short mode")
	}
	ctx := logging.NewRootContext(nil)
	d := &Driver{}

	b.Run("blocks_64", func(b *testing.B) {
		img := loadPaletteImage(b, "blocks_64.png")
		opts := api.Options{Polarity: api.PolarityDark, ColorCount: 16, MaxSamples: 0}
		b.ReportAllocs()
		for b.Loop() {
			if _, err := d.Extract(ctx, img, opts); err != nil {
				b.Fatal(err)
			}
		}
	})
	b.Run("bliss", func(b *testing.B) {
		img := loadPaletteImage(b, "bliss.jpg")
		opts := api.Options{Polarity: api.PolarityDark, ColorCount: 16, MaxSamples: 10000}
		b.ReportAllocs()
		for b.Loop() {
			if _, err := d.Extract(ctx, img, opts); err != nil {
				b.Fatal(err)
			}
		}
	})
}

func BenchmarkGeneticScorePopFromTestdata(b *testing.B) {
	b.Run("blocks_64", func(b *testing.B) {
		img := loadPaletteImage(b, "blocks_64.png")
		colors := api.SampleImage(img, 0)
		lab := make([]api.LAB, len(colors))
		for i, c := range colors {
			lab[i] = api.RGBToLAB(c)
		}
		pop := initPopulation(rand.New(rand.NewSource(1)), 16, 200)
		b.ReportAllocs()
		for b.Loop() {
			_ = scorePop(pop, lab, api.PolarityDark)
		}
	})
	b.Run("bliss_max_10000", func(b *testing.B) {
		img := loadPaletteImage(b, "bliss.jpg")
		colors := api.SampleImage(img, 10000)
		lab := make([]api.LAB, len(colors))
		for i, c := range colors {
			lab[i] = api.RGBToLAB(c)
		}
		pop := initPopulation(rand.New(rand.NewSource(1)), 16, 200)
		b.ReportAllocs()
		for b.Loop() {
			_ = scorePop(pop, lab, api.PolarityDark)
		}
	})
}
