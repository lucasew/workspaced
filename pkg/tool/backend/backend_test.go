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

func TestScoreArtifact(t *testing.T) {
	const (
		osName = "linux"
		arch   = "amd64"
	)

	tests := []struct {
		name string
		art  Artifact
		hint string
		want int
	}{
		{
			name: "empty hint (eligible archive gets positive baseline)",
			art:  Artifact{OS: osName, Arch: arch, URL: "https://example.com/foo.tar.gz"},
			hint: "",
			want: 11, // baseline 1 + 10 for archive
		},
		{
			name: "exact token match",
			art:  Artifact{OS: osName, Arch: arch, URL: "https://example.com/resvg-linux.tar.gz"},
			hint: "resvg",
			want: 190,
		},
		{
			name: "substring match",
			art:  Artifact{OS: osName, Arch: arch, URL: "https://example.com/myresvgtool.tar.gz"},
			hint: "resvg",
			want: 70,
		},
		{
			name: "no name match but good archive",
			art:  Artifact{OS: osName, Arch: arch, URL: "https://example.com/other.tar.gz"},
			hint: "resvg",
			want: 10,
		},
		{
			name: "token match + debug penalty",
			art:  Artifact{OS: osName, Arch: arch, URL: "https://example.com/resvg-debug.tar.gz"},
			hint: "resvg",
			want: 170,
		},

		// Ineligibility cases (must return 0)
		{
			name: "wrong OS",
			art:  Artifact{OS: "darwin", Arch: arch, URL: "https://example.com/resvg-linux.tar.gz"},
			hint: "resvg",
			want: 0,
		},
		{
			name: "wrong arch",
			art:  Artifact{OS: osName, Arch: "arm64", URL: "https://example.com/resvg-linux.tar.gz"},
			hint: "resvg",
			want: 0,
		},
		{
			name: ".deb is ineligible even on matching platform",
			art:  Artifact{OS: osName, Arch: arch, URL: "https://example.com/foo_amd64.deb"},
			hint: "foo",
			want: 0,
		},
		{
			name: ".rpm is ineligible even on matching platform",
			art:  Artifact{OS: osName, Arch: arch, URL: "https://example.com/foo.x86_64.rpm"},
			hint: "foo",
			want: 0,
		},
		{
			name: "no hint + ineligible deb",
			art:  Artifact{OS: osName, Arch: arch, URL: "https://example.com/pkg.deb"},
			hint: "",
			want: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ScoreArtifact(tt.art, osName, arch, tt.hint)
			if got != tt.want {
				t.Errorf("ScoreArtifact(%+v, %q, %q, %q) = %d, want %d",
					tt.art, osName, arch, tt.hint, got, tt.want)
			}
		})
	}
}
