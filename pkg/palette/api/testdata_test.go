package api

import (
	"image"
	_ "image/jpeg"
	_ "image/png"
	"os"
	"path/filepath"
	"testing"
)

// paletteTestdata lives at pkg/palette/testdata (shared by api/genetic/materialyou).
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

func TestSampleImageBlocks64(t *testing.T) {
	t.Parallel()
	img := loadPaletteImage(t, "blocks_64.png")
	// One transparent pixel at (0,0); four opaque quadrant colors.
	colors := SampleImage(img, 0)
	if len(colors) != 4 {
		t.Fatalf("unique opaque colors = %d, want 4", len(colors))
	}

	want := map[uint32]bool{
		0xe53935: true,
		0x43a047: true,
		0x1e88e5: true,
		0xfbc02d: true,
	}
	for _, c := range colors {
		key := uint32(c.R)<<16 | uint32(c.G)<<8 | uint32(c.B)
		if !want[key] {
			t.Errorf("unexpected color #%02x%02x%02x", c.R, c.G, c.B)
		}
		delete(want, key)
	}
	if len(want) != 0 {
		t.Errorf("missing colors: %v", want)
	}
}

func TestSampleImageSolid(t *testing.T) {
	t.Parallel()
	img := loadPaletteImage(t, "solid_4285f4.png")
	colors := SampleImage(img, 0)
	if len(colors) != 1 {
		t.Fatalf("solid unique colors = %d, want 1", len(colors))
	}
	c := colors[0]
	if c.R != 0x42 || c.G != 0x85 || c.B != 0xf4 {
		t.Fatalf("got #%02x%02x%02x, want #4285f4", c.R, c.G, c.B)
	}
}

func TestSampleImageMaxSamplesCapsVisits(t *testing.T) {
	t.Parallel()
	img := loadPaletteImage(t, "gradient_256.png")
	// Full unique set is huge; striding must return fewer or equal uniques and never panic.
	limited := SampleImage(img, 256)
	full := SampleImage(img, 0)
	if len(limited) == 0 {
		t.Fatal("limited sample empty")
	}
	if len(limited) > len(full) {
		t.Fatalf("limited unique %d > full unique %d", len(limited), len(full))
	}
}

// bliss.jpg is the real wallpaper from ~/.dotfiles/assets/wallpapers (4510x3627).
func TestSampleImageBliss(t *testing.T) {
	t.Parallel()
	img := loadPaletteImage(t, "bliss.jpg")
	b := img.Bounds()
	if b.Dx() < 1000 || b.Dy() < 1000 {
		t.Fatalf("unexpected bliss size %dx%d", b.Dx(), b.Dy())
	}
	// CLI default-style budget: must yield some opaque colors, stay bounded.
	colors := SampleImage(img, 10000)
	if len(colors) == 0 {
		t.Fatal("bliss sample empty")
	}
	if len(colors) > 10000 {
		t.Fatalf("unique colors %d exceeds MaxSamples budget", len(colors))
	}
}

func BenchmarkSampleImage(b *testing.B) {
	b.Run("gradient_256/full", func(b *testing.B) {
		img := loadPaletteImage(b, "gradient_256.png")
		b.ReportAllocs()
		for b.Loop() {
			_ = SampleImage(img, 0)
		}
	})
	b.Run("gradient_256/max_10000", func(b *testing.B) {
		img := loadPaletteImage(b, "gradient_256.png")
		b.ReportAllocs()
		for b.Loop() {
			_ = SampleImage(img, 10000)
		}
	})
	b.Run("bliss/max_10000", func(b *testing.B) {
		img := loadPaletteImage(b, "bliss.jpg")
		b.ReportAllocs()
		for b.Loop() {
			_ = SampleImage(img, 10000)
		}
	})
	b.Run("bliss/max_1000", func(b *testing.B) {
		img := loadPaletteImage(b, "bliss.jpg")
		b.ReportAllocs()
		for b.Loop() {
			_ = SampleImage(img, 1000)
		}
	})
}
