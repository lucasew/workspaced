package env_test

import (
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/lucasew/workspaced/pkg/driver/env"
)

func TestMergeEssentialPaths(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name      string
		essential []string
		existing  []string
		want      []string
	}{
		{
			name:      "prepends missing in reverse essential order",
			essential: []string{"A", "B", "C"},
			existing:  []string{"x", "y"},
			want:      []string{"C", "B", "A", "x", "y"},
		},
		{
			name:      "skips essentials already present",
			essential: []string{"A", "B", "C"},
			existing:  []string{"B", "x"},
			want:      []string{"C", "A", "B", "x"},
		},
		{
			name:      "no missing essentials clones existing",
			essential: []string{"A", "B"},
			existing:  []string{"A", "B", "x"},
			want:      []string{"A", "B", "x"},
		},
		{
			name:      "dedupes repeated essentials",
			essential: []string{"A", "A", "B"},
			existing:  []string{"x"},
			want:      []string{"B", "A", "x"},
		},
		{
			name:      "empty essential leaves existing",
			essential: nil,
			existing:  []string{"x"},
			want:      []string{"x"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := env.MergeEssentialPaths(tt.essential, tt.existing)
			if !slices.Equal(got, tt.want) {
				t.Fatalf("MergeEssentialPaths() = %#v, want %#v", got, tt.want)
			}
		})
	}
}

func TestExpandPathInUsesProvidedHome(t *testing.T) {
	t.Parallel()
	got := env.ExpandPathIn("~/.config/workspaced", "/data/home")
	want := filepath.Join("/data/home", ".config/workspaced")
	if got != want {
		t.Fatalf("ExpandPathIn = %q, want %q", got, want)
	}
}

func TestFindDotfilesRoot(t *testing.T) {
	home := t.TempDir()
	root := filepath.Join(home, ".dotfiles")
	if err := os.Mkdir(root, 0o755); err != nil {
		t.Fatal(err)
	}
	got, err := env.FindDotfilesRoot(home)
	if err != nil {
		t.Fatal(err)
	}
	if got != root {
		t.Fatalf("FindDotfilesRoot = %q, want %q", got, root)
	}
}

func TestEnsureUnderHome(t *testing.T) {
	home := t.TempDir()
	got, err := env.EnsureUnderHome(home, ".local/share/workspaced")
	if err != nil {
		t.Fatal(err)
	}
	want := filepath.Join(home, ".local/share/workspaced")
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
	if _, err := os.Stat(got); err != nil {
		t.Fatal(err)
	}
}
