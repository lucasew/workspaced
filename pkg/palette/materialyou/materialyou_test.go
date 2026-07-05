package materialyou

import (
	"image"
	"image/color"
	"testing"

	"workspaced/pkg/logging"
	"workspaced/pkg/palette/api"
)

func TestMaterialYouDriver(t *testing.T) {
	t.Parallel()
	d := &Driver{}
	if d.Name() != "materialyou" {
		t.Errorf("expected materialyou, got %s", d.Name())
	}

	img := image.NewRGBA(image.Rect(0, 0, 1, 1))
	img.SetRGBA(0, 0, color.RGBA{66, 133, 244, 255}) // #4285F4

	ctx := logging.NewRootContext(nil)

	pal, err := d.Extract(ctx, img, api.Options{Polarity: api.PolarityDark, ColorCount: 16})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if pal.Base00 == "" {
		t.Error("expected Base00 to be set")
	}
	if pal.Base10 != "" {
		t.Errorf("expected Base10 to be empty for ColorCount 16, got %s", pal.Base10)
	}
	if len(pal.Base00) != 6 || pal.Base00[0] == '#' {
		t.Errorf("expected 6-digit hex without '#', got %q", pal.Base00)
	}

	pal24, err := d.Extract(ctx, img, api.Options{Polarity: api.PolarityLight, ColorCount: 24})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if pal24.Base10 == "" {
		t.Error("expected Base10 to be set for ColorCount 24")
	}
}

func TestMaterialYouGolden(t *testing.T) {
	t.Parallel()
	img := image.NewRGBA(image.Rect(0, 0, 1, 1))
	img.SetRGBA(0, 0, color.RGBA{66, 133, 244, 255})
	ctx := logging.NewRootContext(nil)
	d := &Driver{}

	dark, err := d.Extract(ctx, img, api.Options{Polarity: api.PolarityDark, ColorCount: 16})
	if err != nil {
		t.Fatal(err)
	}
	wantDark := &api.Palette{
		Base00: "131313", Base01: "131313", Base02: "1f1f1f", Base03: "2a2a2a",
		Base04: "474747", Base05: "e2e2e2", Base06: "c6c6c6", Base07: "e2e2e2",
		Base08: "ffb4ab", Base09: "bfc6dc", Base0A: "debcdf", Base0B: "adc6ff",
		Base0C: "0f448e", Base0D: "3f4759", Base0E: "583e5b", Base0F: "93000a",
	}
	if *dark != *wantDark {
		t.Fatalf("dark16 mismatch\ngot  %#v\nwant %#v", dark, wantDark)
	}

	light, err := d.Extract(ctx, img, api.Options{Polarity: api.PolarityLight, ColorCount: 24})
	if err != nil {
		t.Fatal(err)
	}
	wantLight := &api.Palette{
		Base00: "f9f9f9", Base01: "dadada", Base02: "eeeeee", Base03: "e8e8e8",
		Base04: "e2e2e2", Base05: "1b1b1b", Base06: "474747", Base07: "303030",
		Base08: "ba1a1a", Base09: "575e71", Base0A: "715573", Base0B: "315da8",
		Base0C: "d8e2ff", Base0D: "dbe2f9", Base0E: "fbd7fc", Base0F: "ffdad6",
		Base10: "f9f9f9", Base11: "ffffff", Base12: "f3f3f3", Base13: "e2e2e2",
		Base14: "777777", Base15: "c6c6c6", Base16: "adc6ff", Base17: "f1f1f1",
	}
	if *light != *wantLight {
		t.Fatalf("light24 mismatch\ngot  %#v\nwant %#v", light, wantLight)
	}
}
