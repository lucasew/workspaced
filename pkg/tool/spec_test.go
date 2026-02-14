package tool

import (
	"testing"
)

func TestParseToolSpec(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    ToolSpec
		wantErr bool
	}{
		{
			name:  "full spec with version",
			input: "github:denoland/deno@1.40.0",
			want: ToolSpec{
				Provider: "github",
				Package:  "denoland/deno",
				Version:  "1.40.0",
			},
			wantErr: false,
		},
		{
			name:  "spec with latest",
			input: "github:denoland/deno@latest",
			want: ToolSpec{
				Provider: "github",
				Package:  "denoland/deno",
				Version:  "latest",
			},
			wantErr: false,
		},
		{
			name:  "spec without version defaults to latest",
			input: "github:denoland/deno",
			want: ToolSpec{
				Provider: "github",
				Package:  "denoland/deno",
				Version:  "latest",
			},
			wantErr: false,
		},
		{
			name:  "spec with v prefix in version",
			input: "github:golang/go@v1.21.0",
			want: ToolSpec{
				Provider: "github",
				Package:  "golang/go",
				Version:  "v1.21.0",
			},
			wantErr: false,
		},
		{
			name:  "simple package name",
			input: "github:ripgrep@14.0.0",
			want: ToolSpec{
				Provider: "github",
				Package:  "ripgrep",
				Version:  "14.0.0",
			},
			wantErr: false,
		},
		{
			name:  "omit provider uses github default",
			input: "denoland/deno@1.40.0",
			want: ToolSpec{
				Provider: "github",
				Package:  "denoland/deno",
				Version:  "1.40.0",
			},
			wantErr: false,
		},
		{
			name:  "omit both provider and version",
			input: "deno",
			want: ToolSpec{
				Provider: "github",
				Package:  "deno",
				Version:  "latest",
			},
			wantErr: false,
		},
		{
			name:  "omit provider with version",
			input: "ripgrep@14.0.0",
			want: ToolSpec{
				Provider: "github",
				Package:  "ripgrep",
				Version:  "14.0.0",
			},
			wantErr: false,
		},
		{
			name:    "empty string",
			input:   "",
			want:    ToolSpec{},
			wantErr: true,
		},
		{
			name:  "only provider colon creates empty package",
			input: "github:",
			want: ToolSpec{
				Provider: "github",
				Package:  "",
				Version:  "latest",
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseToolSpec(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseToolSpec() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && got != tt.want {
				t.Errorf("ParseToolSpec() = %+v, want %+v", got, tt.want)
			}
		})
	}
}

func TestToolSpec_String(t *testing.T) {
	tests := []struct {
		name string
		spec ToolSpec
		want string
	}{
		{
			name: "full spec",
			spec: ToolSpec{
				Provider: "github",
				Package:  "denoland/deno",
				Version:  "1.40.0",
			},
			want: "github:denoland/deno@1.40.0",
		},
		{
			name: "latest version",
			spec: ToolSpec{
				Provider: "github",
				Package:  "golang/go",
				Version:  "latest",
			},
			want: "github:golang/go@latest",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.spec.String(); got != tt.want {
				t.Errorf("ToolSpec.String() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestSpecToDir(t *testing.T) {
	tests := []struct {
		name       string
		providerID string
		pkgSpec    string
		want       string
	}{
		{
			name:       "github package with slash",
			providerID: "github",
			pkgSpec:    "denoland/deno",
			want:       "github-denoland-deno",
		},
		{
			name:       "simple package name",
			providerID: "github",
			pkgSpec:    "ripgrep",
			want:       "github-ripgrep",
		},
		{
			name:       "nested slashes",
			providerID: "gitlab",
			pkgSpec:    "group/subgroup/project",
			want:       "gitlab-group-subgroup-project",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := SpecToDir(tt.providerID, tt.pkgSpec); got != tt.want {
				t.Errorf("SpecToDir() = %v, want %v", got, tt.want)
			}
		})
	}
}
