package tool

import (
	"path/filepath"
	"testing"
)

func TestBinaryCandidates(t *testing.T) {
	tests := []struct {
		name    string
		baseDir string
		cmdName string
		want    []string
	}{
		{
			name:    "standard layout",
			baseDir: "/tools/github-cli-cli/2.0.0",
			cmdName: "gh",
			want: []string{
				filepath.Join("/tools/github-cli-cli/2.0.0", "bin", "gh"),
				filepath.Join("/tools/github-cli-cli/2.0.0", "bin", "gh.exe"),
				filepath.Join("/tools/github-cli-cli/2.0.0", "gh"),
				filepath.Join("/tools/github-cli-cli/2.0.0", "gh.exe"),
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := BinaryCandidates(tt.baseDir, tt.cmdName)
			if len(got) != len(tt.want) {
				t.Fatalf("len(BinaryCandidates) = %d, want %d", len(got), len(tt.want))
			}
			for i := range got {
				if got[i] != tt.want[i] {
					t.Errorf("BinaryCandidates[%d] = %q, want %q", i, got[i], tt.want[i])
				}
			}
		})
	}
}

func TestNormalizeVersion(t *testing.T) {
	tests := []struct {
		name    string
		version string
		want    string
	}{
		{name: "v prefix", version: "v1.2.3", want: "1.2.3"},
		{name: "no prefix", version: "1.2.3", want: "1.2.3"},
		{name: "with slash", version: "refs/heads/main", want: "refs-heads-main"},
		{name: "v prefix with slash", version: "v1.0/rc1", want: "1.0-rc1"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := normalizeVersion(tt.version)
			if got != tt.want {
				t.Errorf("normalizeVersion(%q) = %q, want %q", tt.version, got, tt.want)
			}
		})
	}
}
