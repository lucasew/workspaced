package materialyou

import (
	"encoding/json"
	"image"
	_ "image/jpeg"
	_ "image/png"
	"os"
	"path/filepath"
	"testing"

	"github.com/lucasew/workspaced/pkg/logging"
	"github.com/lucasew/workspaced/pkg/palette/api"
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

func loadGoldenPalette(t testing.TB, name string) *api.Palette {
	t.Helper()
	b, err := os.ReadFile(paletteTestdata(t, name))
	if err != nil {
		t.Fatalf("read golden %s: %v", name, err)
	}
	var p api.Palette
	if err := json.Unmarshal(b, &p); err != nil {
		t.Fatalf("parse golden %s: %v", name, err)
	}
	return &p
}

func TestMaterialYouFromTestdataSolid(t *testing.T) {
	t.Parallel()
	img := loadPaletteImage(t, "solid_4285f4.png")
	ctx := logging.NewWriterContext(t.Output())
	d := &Driver{}

	dark, err := d.Extract(ctx, img, api.Options{Polarity: api.PolarityDark, ColorCount: 16})
	if err != nil {
		t.Fatal(err)
	}
	wantDark := loadGoldenPalette(t, "materialyou_dark16_4285f4.json")
	if *dark != *wantDark {
		t.Fatalf("dark16 mismatch\ngot  %#v\nwant %#v", dark, wantDark)
	}

	light, err := d.Extract(ctx, img, api.Options{Polarity: api.PolarityLight, ColorCount: 24})
	if err != nil {
		t.Fatal(err)
	}
	wantLight := loadGoldenPalette(t, "materialyou_light24_4285f4.json")
	if *light != *wantLight {
		t.Fatalf("light24 mismatch\ngot  %#v\nwant %#v", light, wantLight)
	}
}

func TestGenerateColorschemeSourceHex(t *testing.T) {
	t.Parallel()
	// Same dominant color as solid_4285f4.png / golden fixtures.
	scheme := GenerateColorscheme("#4285f4", nil)
	if scheme.Dark["surface"] == "" || scheme.Light["primary"] == "" {
		t.Fatalf("incomplete scheme: dark.surface=%q light.primary=%q",
			scheme.Dark["surface"], scheme.Light["primary"])
	}
	// Extract maps surface → base00 for dark; golden locks that slot.
	want := loadGoldenPalette(t, "materialyou_dark16_4285f4.json")
	if got := scheme.Dark["surface"]; got != "#"+want.Base00 && got != want.Base00 {
		// colorsFor returns with or without # depending on Tone(); normalize
		g := got
		if len(g) == 7 && g[0] == '#' {
			g = g[1:]
		}
		if g != want.Base00 {
			t.Fatalf("dark surface = %q, want %q (from golden base00)", got, want.Base00)
		}
	}
}

// bliss.jpg from ~/.dotfiles/assets/wallpapers — realistic multi-color source.
// MaxSamples must match the golden generator (CLI default 10000).
func TestMaterialYouFromTestdataBliss(t *testing.T) {
	t.Parallel()
	img := loadPaletteImage(t, "bliss.jpg")
	ctx := logging.NewWriterContext(t.Output())
	d := &Driver{}
	opts := api.Options{Polarity: api.PolarityDark, ColorCount: 16, MaxSamples: 10000}

	dark, err := d.Extract(ctx, img, opts)
	if err != nil {
		t.Fatal(err)
	}
	wantDark := loadGoldenPalette(t, "materialyou_dark16_bliss.json")
	if *dark != *wantDark {
		t.Fatalf("bliss dark16 mismatch\ngot  %#v\nwant %#v", dark, wantDark)
	}

	light, err := d.Extract(ctx, img, api.Options{Polarity: api.PolarityLight, ColorCount: 24, MaxSamples: 10000})
	if err != nil {
		t.Fatal(err)
	}
	wantLight := loadGoldenPalette(t, "materialyou_light24_bliss.json")
	if *light != *wantLight {
		t.Fatalf("bliss light24 mismatch\ngot  %#v\nwant %#v", light, wantLight)
	}
}

func BenchmarkMaterialYouExtract(b *testing.B) {
	ctx := logging.NewWriterContext(b.Output())
	d := &Driver{}
	opts := api.Options{Polarity: api.PolarityDark, ColorCount: 16, MaxSamples: 10000}

	b.Run("gradient_256", func(b *testing.B) {
		img := loadPaletteImage(b, "gradient_256.png")
		b.ReportAllocs()
		for b.Loop() {
			if _, err := d.Extract(ctx, img, opts); err != nil {
				b.Fatal(err)
			}
		}
	})
	b.Run("bliss", func(b *testing.B) {
		img := loadPaletteImage(b, "bliss.jpg")
		b.ReportAllocs()
		for b.Loop() {
			if _, err := d.Extract(ctx, img, opts); err != nil {
				b.Fatal(err)
			}
		}
	})
}

func BenchmarkGenerateColorscheme(b *testing.B) {
	b.ReportAllocs()
	for b.Loop() {
		_ = GenerateColorscheme("#4285f4", nil)
	}
}
