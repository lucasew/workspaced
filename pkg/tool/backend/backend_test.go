package backend

import "testing"

func TestContainsAnyOf(t *testing.T) {
	tests := []struct {
		name     string
		haystack string
		needles  []string
		want     bool
	}{
		{name: "match first", haystack: "linux-amd64.tar.gz", needles: []string{"linux", "darwin"}, want: true},
		{name: "match second", haystack: "darwin-arm64.tar.gz", needles: []string{"linux", "darwin"}, want: true},
		{name: "no match", haystack: "windows-x86.zip", needles: []string{"linux", "darwin"}, want: false},
		{name: "empty needles", haystack: "anything", needles: nil, want: false},
		{name: "empty haystack", haystack: "", needles: []string{"linux"}, want: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ContainsAnyOf(tt.haystack, tt.needles...)
			if got != tt.want {
				t.Errorf("ContainsAnyOf(%q, %v) = %v, want %v", tt.haystack, tt.needles, got, tt.want)
			}
		})
	}
}

func TestScoreArtifactForHint(t *testing.T) {
	tests := []struct {
		name string
		url  string
		hint string
		want int
	}{
		{name: "empty hint", url: "https://example.com/foo.tar.gz", hint: "", want: 0},
		{name: "exact token match", url: "https://example.com/resvg-linux.tar.gz", hint: "resvg", want: 190},
		{name: "substring match", url: "https://example.com/myresvgtool.tar.gz", hint: "resvg", want: 70},
		{name: "no match", url: "https://example.com/other.tar.gz", hint: "resvg", want: 10},
		{name: "debug penalty", url: "https://example.com/resvg-debug.tar.gz", hint: "resvg", want: 170},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := scoreArtifactForHint(tt.url, tt.hint)
			if got != tt.want {
				t.Errorf("scoreArtifactForHint(%q, %q) = %d, want %d", tt.url, tt.hint, got, tt.want)
			}
		})
	}
}
