package icons

import (
	"encoding/json"
	"image"
	"image/color"
	_ "image/png"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func iconTestdata(t testing.TB, name string) string {
	t.Helper()
	return filepath.Join("testdata", name)
}

func loadIconImage(t testing.TB, name string) image.Image {
	t.Helper()
	f, err := os.Open(iconTestdata(t, name))
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

func loadBase16Fixture(t testing.TB) map[string]string {
	t.Helper()
	b, err := os.ReadFile(iconTestdata(t, "base16_fixture.json"))
	if err != nil {
		t.Fatal(err)
	}
	var raw map[string]string
	if err := json.Unmarshal(b, &raw); err != nil {
		t.Fatal(err)
	}
	// renderSVG also indexes UPPER keys
	out := make(map[string]string, len(raw)*2)
	for k, v := range raw {
		v = strings.TrimPrefix(v, "#")
		out[k] = v
		out[strings.ToUpper(k)] = v
	}
	return out
}

func nrgbaAt(img image.Image, x, y int) color.NRGBA {
	r, g, b, a := img.At(x, y).RGBA()
	return color.NRGBA{uint8(r >> 8), uint8(g >> 8), uint8(b >> 8), uint8(a >> 8)}
}

func TestMakeBackgroundTransparentFloodFill(t *testing.T) {
	t.Parallel()
	img := loadIconImage(t, "flood_fill_64.png")
	out := makeBackgroundTransparent(img)

	// Corners were opaque green background → transparent after flood fill.
	for _, p := range [][2]int{{0, 0}, {63, 0}, {0, 63}, {63, 63}} {
		c := nrgbaAt(out, p[0], p[1])
		if c.A != 0 {
			t.Fatalf("corner (%d,%d) alpha=%d, want 0", p[0], p[1], c.A)
		}
	}
	// Center of red square must remain opaque red-ish.
	c := nrgbaAt(out, 32, 32)
	if c.A == 0 {
		t.Fatal("center became transparent")
	}
	if c.R < 0x80 || c.G > 0x40 {
		t.Fatalf("center color unexpected: %#v", c)
	}
}

func TestMakeBackgroundTransparentAlreadyClear(t *testing.T) {
	t.Parallel()
	img := loadIconImage(t, "transparent_bg_32.png")
	out := makeBackgroundTransparent(img)
	// Early-return path: same image value when corner already transparent.
	if out != img {
		t.Fatal("expected identity return when background already transparent")
	}
}

func TestCropToContentSquare(t *testing.T) {
	t.Parallel()
	img := loadIconImage(t, "transparent_bg_32.png")
	out := cropToContentSquare(img)
	b := out.Bounds()
	// Content was 16x16 centered in 32x32 → square crop of content.
	if b.Dx() != b.Dy() {
		t.Fatalf("expected square, got %dx%d", b.Dx(), b.Dy())
	}
	if b.Dx() < 16 || b.Dx() > 32 {
		t.Fatalf("unexpected crop size %d", b.Dx())
	}
	// Cropped image should have some opaque pixels.
	opaque := 0
	for y := b.Min.Y; y < b.Max.Y; y++ {
		for x := b.Min.X; x < b.Max.X; x++ {
			if _, _, _, a := out.At(x, y).RGBA(); a > 0 {
				opaque++
			}
		}
	}
	if opaque == 0 {
		t.Fatal("crop produced fully transparent image")
	}
}

func TestResizeAndCenter(t *testing.T) {
	t.Parallel()
	img := loadIconImage(t, "wide_48x24.png")
	resized := resizeBilinear(img, 24, 12)
	if resized.Bounds().Dx() != 24 || resized.Bounds().Dy() != 12 {
		t.Fatalf("resize bounds = %v", resized.Bounds())
	}
	squared := centerInSquare(resized, 32)
	if squared.Bounds().Dx() != 32 || squared.Bounds().Dy() != 32 {
		t.Fatalf("center bounds = %v", squared.Bounds())
	}
}

func TestRenderSVGReplaceAndTemplate(t *testing.T) {
	t.Parallel()
	colors := loadBase16Fixture(t)

	// Explicit replace: red → base08 from fixture.
	out, err := renderSVG(iconTestdata(t, "apps/sample.svg"), colors, map[string]string{
		"ff0000": colors["base08"],
	}, false, "test-theme", "sample")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "#"+colors["base08"]) {
		t.Fatalf("expected replaced fill with base08 #%s, got:\n%s", colors["base08"], out)
	}
	if strings.Contains(strings.ToLower(out), "#ff0000") {
		t.Fatalf("old #ff0000 still present:\n%s", out)
	}

	// Template path: {{.base00}} / {{.base0D}}
	tmplOut, err := renderSVG(iconTestdata(t, "apps/templated.svg.tmpl"), colors, nil, false, "test-theme", "templated")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(tmplOut, "#"+colors["base00"]) || !strings.Contains(tmplOut, "#"+colors["base0D"]) {
		t.Fatalf("template did not expand base slots:\n%s", tmplOut)
	}
	if strings.Contains(tmplOut, "{{") {
		t.Fatalf("unexpanded template left in output:\n%s", tmplOut)
	}
}

func TestMapHexColorsToScheme(t *testing.T) {
	t.Parallel()
	colors := loadBase16Fixture(t)
	in, err := os.ReadFile(iconTestdata(t, "placeholder.svg"))
	if err != nil {
		t.Fatal(err)
	}
	out := mapHexColorsToScheme(string(in), colors)
	if strings.Contains(strings.ToLower(out), "#abcdef") {
		t.Fatalf("source hex not mapped:\n%s", out)
	}
	// Nearest palette color should be a known base16 hex.
	found := false
	for k, v := range colors {
		if strings.HasPrefix(k, "base") && strings.Contains(strings.ToLower(out), "#"+strings.ToLower(v)) {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("mapped output has no fixture palette color:\n%s", out)
	}
}

func TestCollectIconInputsTestdata(t *testing.T) {
	t.Parallel()
	paths, err := collectIconInputs(iconTestdata(t, "."))
	if err != nil {
		t.Fatal(err)
	}
	if len(paths) < 3 {
		t.Fatalf("expected >=3 svg inputs, got %d: %v", len(paths), paths)
	}
}

func BenchmarkMakeBackgroundTransparent(b *testing.B) {
	img := loadIconImage(b, "flood_fill_64.png")
	b.ReportAllocs()
	for b.Loop() {
		_ = makeBackgroundTransparent(img)
	}
}

func BenchmarkCropToContentSquare(b *testing.B) {
	img := loadIconImage(b, "transparent_bg_32.png")
	b.ReportAllocs()
	for b.Loop() {
		_ = cropToContentSquare(img)
	}
}

func BenchmarkResizeBilinear(b *testing.B) {
	img := loadIconImage(b, "wide_48x24.png")
	b.ReportAllocs()
	for b.Loop() {
		_ = resizeBilinear(img, 128, 64)
	}
}

func BenchmarkCenterInSquare(b *testing.B) {
	img := loadIconImage(b, "wide_48x24.png")
	b.ReportAllocs()
	for b.Loop() {
		_ = centerInSquare(img, 64)
	}
}

func BenchmarkRenderSVGMapScheme(b *testing.B) {
	colors := loadBase16Fixture(b)
	path := iconTestdata(b, "apps/sample.svg")
	b.ReportAllocs()
	for b.Loop() {
		if _, err := renderSVG(path, colors, nil, true, "bench-theme", "sample"); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkMapHexColorsToScheme(b *testing.B) {
	colors := loadBase16Fixture(b)
	in, err := os.ReadFile(iconTestdata(b, "apps/sample.svg"))
	if err != nil {
		b.Fatal(err)
	}
	s := string(in)
	b.ReportAllocs()
	for b.Loop() {
		_ = mapHexColorsToScheme(s, colors)
	}
}
