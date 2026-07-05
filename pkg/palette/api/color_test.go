package api

import (
	"image/color"
	"testing"
)

func TestNormalizeHex(t *testing.T) {
	t.Parallel()
	cases := []struct {
		in, want string
	}{
		{"", ""},
		{"#aabbcc", "aabbcc"},
		{"aabbcc", "aabbcc"},
		{"#", ""},
	}
	for _, tc := range cases {
		if got := NormalizeHex(tc.in); got != tc.want {
			t.Errorf("NormalizeHex(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

func TestPaletteFromHexes(t *testing.T) {
	t.Parallel()
	p := PaletteFromHexes([]string{"#112233", "445566"})
	if p.Base00 != "112233" || p.Base01 != "445566" {
		t.Fatalf("unexpected base slots: base00=%q base01=%q", p.Base00, p.Base01)
	}
	if p.Base02 != "" || p.Base10 != "" {
		t.Fatalf("expected unset slots empty, got base02=%q base10=%q", p.Base02, p.Base10)
	}

	full := make([]string, 24)
	for i := range full {
		full[i] = "000000"
	}
	full[0] = "#ffffff"
	full[16] = "#abcdef"
	p24 := PaletteFromHexes(full)
	if p24.Base00 != "ffffff" || p24.Base10 != "abcdef" || p24.Base17 != "000000" {
		t.Fatalf("base24 mapping wrong: base00=%q base10=%q base17=%q", p24.Base00, p24.Base10, p24.Base17)
	}

	overflow := make([]string, 0, 25)
	overflow = append(overflow, full...)
	overflow = append(overflow, "deadbeef")
	pExtra := PaletteFromHexes(overflow)
	if pExtra.Base17 != "000000" {
		t.Fatalf("entries beyond 24 should be ignored, base17=%q", pExtra.Base17)
	}
}

func TestToHex(t *testing.T) {
	t.Parallel()
	got := ToHex(color.RGBA{R: 0x42, G: 0x85, B: 0xf4, A: 0xff})
	if got != "4285f4" {
		t.Fatalf("ToHex = %q, want 4285f4", got)
	}
}

func TestRGBToLABRoundTripLightness(t *testing.T) {
	t.Parallel()
	c := color.RGBA{R: 255, G: 255, B: 255, A: 255}
	lab := RGBToLAB(c)
	if lab.L < 99 || lab.L > 100.1 {
		t.Fatalf("white L* = %v, want ~100", lab.L)
	}
	back := LABToRGB(lab)
	if back.R < 250 || back.G < 250 || back.B < 250 {
		t.Fatalf("round-trip white got %#v", back)
	}
}
