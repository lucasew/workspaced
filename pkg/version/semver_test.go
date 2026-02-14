package version

import (
	"sort"
	"testing"
)

func TestParseVersion(t *testing.T) {
	tests := []struct {
		input string
		want  SemVer
	}{
		{
			input: "1.2.3",
			want:  SemVer{Original: "1.2.3", Parts: []int{1, 2, 3}},
		},
		{
			input: "v1.2.3",
			want:  SemVer{Original: "v1.2.3", Parts: []int{1, 2, 3}},
		},
		{
			input: "2.0.0",
			want:  SemVer{Original: "2.0.0", Parts: []int{2, 0, 0}},
		},
		{
			input: "latest",
			want:  SemVer{Original: "latest", Parts: nil},
		},
		{
			input: "1.0.0-beta",
			want:  SemVer{Original: "1.0.0-beta", Parts: []int{1, 0, 0}},
		},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := Parse(tt.input)
			if got.Original != tt.want.Original {
				t.Errorf("Original = %v, want %v", got.Original, tt.want.Original)
			}
			if len(got.Parts) != len(tt.want.Parts) {
				t.Errorf("Parts length = %v, want %v", len(got.Parts), len(tt.want.Parts))
				return
			}
			for i := range got.Parts {
				if got.Parts[i] != tt.want.Parts[i] {
					t.Errorf("Parts[%d] = %v, want %v", i, got.Parts[i], tt.want.Parts[i])
				}
			}
		})
	}
}

func TestVersion_String(t *testing.T) {
	tests := []struct {
		version SemVer
		want    string
	}{
		{Parse("v1.2.3"), "v1.2.3"},
		{Parse("1.2.3"), "1.2.3"},
		{Parse("latest"), "latest"},
	}

	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			if got := tt.version.String(); got != tt.want {
				t.Errorf("String() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestVersion_Normalized(t *testing.T) {
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
			v := Parse(tt.input)
			if got := v.Normalized(); got != tt.want {
				t.Errorf("Normalized() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestVersion_Compare(t *testing.T) {
	tests := []struct {
		v1   string
		v2   string
		want int
	}{
		{"1.2.3", "1.2.3", 0},
		{"1.2.3", "1.2.4", -1},
		{"1.2.4", "1.2.3", 1},
		{"2.0.0", "1.9.9", 1},
		{"1.0.0", "1.0.0.1", -1},
		{"v1.2.3", "1.2.3", 0},
		{"latest", "1.0.0", 1},
		{"1.0.0", "latest", -1},
		{"latest", "latest", 0},
	}

	for _, tt := range tests {
		t.Run(tt.v1+"_vs_"+tt.v2, func(t *testing.T) {
			v1 := Parse(tt.v1)
			v2 := Parse(tt.v2)
			got := v1.Compare(v2)
			if got != tt.want {
				t.Errorf("Compare(%v, %v) = %v, want %v", tt.v1, tt.v2, got, tt.want)
			}
		})
	}
}

func TestVersions_Sort(t *testing.T) {
	versions := SemVers{
		Parse("1.0.0"),
		Parse("2.0.0"),
		Parse("1.5.0"),
		Parse("v1.2.0"),
		Parse("latest"),
	}

	sort.Sort(versions)

	// After sorting ascending: 1.0.0, 1.2.0, 1.5.0, 2.0.0, latest
	expected := []string{"1.0.0", "v1.2.0", "1.5.0", "2.0.0", "latest"}
	for i, want := range expected {
		if versions[i].String() != want {
			t.Errorf("versions[%d] = %v, want %v", i, versions[i].String(), want)
		}
	}
}

func TestVersions_SortDescending(t *testing.T) {
	versions := SemVers{
		Parse("1.0.0"),
		Parse("2.0.0"),
		Parse("1.5.0"),
		Parse("v1.2.0"),
	}

	sort.Sort(sort.Reverse(versions))

	// After sorting descending: 2.0.0, 1.5.0, 1.2.0, 1.0.0
	expected := []string{"2.0.0", "1.5.0", "v1.2.0", "1.0.0"}
	for i, want := range expected {
		if versions[i].String() != want {
			t.Errorf("versions[%d] = %v, want %v", i, versions[i].String(), want)
		}
	}
}
