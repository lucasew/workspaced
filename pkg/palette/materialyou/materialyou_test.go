package materialyou

import (
	"image"
	"image/color"
	"os/exec"
	"testing"

	"workspaced/pkg/logging"
	"workspaced/pkg/palette/api"
)

func TestMaterialYouCrossNix(t *testing.T) {
	_, err := exec.LookPath("nix-instantiate")
	if err != nil {
		t.Skip("nix-instantiate not found, skipping Nix cross-test")
	}
}

func TestMaterialYouDriver(t *testing.T) {
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

	pal24, err := d.Extract(ctx, img, api.Options{Polarity: api.PolarityLight, ColorCount: 24})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if pal24.Base10 == "" {
		t.Error("expected Base10 to be set for ColorCount 24")
	}
}
