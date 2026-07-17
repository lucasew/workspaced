package semver

import "testing"

func TestParse(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		wantLen  int
		wantOrig string
	}{
		{name: "simple", input: "1.2.3", wantLen: 3, wantOrig: "1.2.3"},
		{name: "v prefix", input: "v1.2.3", wantLen: 3, wantOrig: "v1.2.3"},
		{name: "latest", input: "latest", wantLen: 0, wantOrig: "latest"},
		{name: "prerelease", input: "1.0.0-beta", wantLen: 3, wantOrig: "1.0.0-beta"},
		{name: "two parts", input: "3.14", wantLen: 2, wantOrig: "3.14"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Parse(tt.input)
			if got.Original != tt.wantOrig {
				t.Errorf("Original = %q, want %q", got.Original, tt.wantOrig)
			}
			if len(got.Parts) != tt.wantLen {
				t.Errorf("len(Parts) = %d, want %d", len(got.Parts), tt.wantLen)
			}
		})
	}
}

func TestCompare(t *testing.T) {
	tests := []struct {
		name string
		a, b string
		want int
	}{
		{name: "equal", a: "1.2.3", b: "1.2.3", want: 0},
		{name: "less", a: "1.2.3", b: "1.2.4", want: -1},
		{name: "greater", a: "2.0.0", b: "1.9.9", want: 1},
		{name: "v prefix equal", a: "v1.0.0", b: "1.0.0", want: 0},
		{name: "latest vs version", a: "latest", b: "99.0.0", want: 1},
		{name: "version vs latest", a: "1.0.0", b: "latest", want: -1},
		{name: "both latest", a: "latest", b: "latest", want: 0},
		{name: "different lengths", a: "1.0", b: "1.0.0", want: 0},
		{name: "different lengths unequal", a: "1.0", b: "1.0.1", want: -1},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Parse(tt.a).Compare(Parse(tt.b))
			if got != tt.want {
				t.Errorf("Compare(%q, %q) = %d, want %d", tt.a, tt.b, got, tt.want)
			}
		})
	}
}

func TestNormalized(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"v1.2.3", "1.2.3"},
		{"1.2.3", "1.2.3"},
		{"latest", "latest"},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := Parse(tt.input).Normalized()
			if got != tt.want {
				t.Errorf("Normalized() = %q, want %q", got, tt.want)
			}
		})
	}
}
