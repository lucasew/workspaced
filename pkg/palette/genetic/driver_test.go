package genetic

import (
	"testing"

	"github.com/lucasew/workspaced/pkg/palette/api"
)

func TestMapToPaletteCounts(t *testing.T) {
	t.Parallel()
	colors := make([]api.LAB, 24)
	for i := range colors {
		colors[i] = api.LAB{L: float64(i), A: 0, B: 0}
	}
	ind := Individual{colors: colors}

	p16 := mapToPalette(ind, 16)
	if p16.Base00 == "" || p16.Base0F == "" {
		t.Fatalf("base16 slots empty: base00=%q base0F=%q", p16.Base00, p16.Base0F)
	}
	if p16.Base10 != "" {
		t.Fatalf("base10 should be empty for colorCount 16, got %q", p16.Base10)
	}
	if p16.Base00[0] == '#' {
		t.Fatalf("hex should not include '#': %q", p16.Base00)
	}

	p24 := mapToPalette(ind, 24)
	if p24.Base10 == "" || p24.Base17 == "" {
		t.Fatalf("base24 slots empty: base10=%q base17=%q", p24.Base10, p24.Base17)
	}

	short := mapToPalette(Individual{colors: colors[:8]}, 16)
	if short.Base00 != "" || short.Base0F != "" {
		t.Fatalf("short individual should yield empty palette, got base00=%q", short.Base00)
	}
}
