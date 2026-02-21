package spec

import "testing"

func TestParse(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    Spec
		wantErr bool
	}{
		{
			name:  "full spec with version",
			input: "github:denoland/deno@1.40.0",
			want: Spec{
				Provider: "github",
				Package:  "denoland/deno",
				Version:  "1.40.0",
			},
		},
		{
			name:  "spec with latest",
			input: "github:denoland/deno@latest",
			want: Spec{
				Provider: "github",
				Package:  "denoland/deno",
				Version:  "latest",
			},
		},
		{
			name:  "spec without version defaults to latest",
			input: "github:denoland/deno",
			want: Spec{
				Provider: "github",
				Package:  "denoland/deno",
				Version:  "latest",
			},
		},
		{
			name:  "spec with v prefix in version",
			input: "github:golang/go@v1.21.0",
			want: Spec{
				Provider: "github",
				Package:  "golang/go",
				Version:  "v1.21.0",
			},
		},
		{
			name:  "simple package name",
			input: "github:ripgrep@14.0.0",
			want: Spec{
				Provider: "github",
				Package:  "ripgrep",
				Version:  "14.0.0",
			},
		},
		{
			name:  "omit provider uses registry default",
			input: "denoland/deno@1.40.0",
			want: Spec{
				Provider: "registry",
				Package:  "denoland/deno",
				Version:  "1.40.0",
			},
		},
		{
			name:  "omit both provider and version",
			input: "deno",
			want: Spec{
				Provider: "registry",
				Package:  "deno",
				Version:  "latest",
			},
		},
		{
			name:  "omit provider with version",
			input: "ripgrep@14.0.0",
			want: Spec{
				Provider: "registry",
				Package:  "ripgrep",
				Version:  "14.0.0",
			},
		},
		{
			name:    "empty string",
			input:   "",
			want:    Spec{},
			wantErr: true,
		},
		{
			name:  "only provider colon creates empty package",
			input: "registry:",
			want: Spec{
				Provider: "registry",
				Package:  "",
				Version:  "latest",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := Parse(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("Parse() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && got != tt.want {
				t.Errorf("Parse() = %+v, want %+v", got, tt.want)
			}
		})
	}
}

func TestSpecString(t *testing.T) {
	tests := []struct {
		name string
		spec Spec
		want string
	}{
		{
			name: "full spec",
			spec: Spec{
				Provider: "github",
				Package:  "denoland/deno",
				Version:  "1.40.0",
			},
			want: "github:denoland/deno@1.40.0",
		},
		{
			name: "latest version",
			spec: Spec{
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
				t.Errorf("Spec.String() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestSpecDir(t *testing.T) {
	tests := []struct {
		name string
		spec Spec
		want string
	}{
		{
			name: "github package with slash",
			spec: Spec{
				Provider: "github",
				Package:  "denoland/deno",
			},
			want: "github-denoland-deno",
		},
		{
			name: "simple package name",
			spec: Spec{
				Provider: "github",
				Package:  "ripgrep",
			},
			want: "github-ripgrep",
		},
		{
			name: "nested slashes",
			spec: Spec{
				Provider: "gitlab",
				Package:  "group/subgroup/project",
			},
			want: "gitlab-group-subgroup-project",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.spec.Dir(); got != tt.want {
				t.Errorf("Spec.Dir() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestToDir(t *testing.T) {
	tests := []struct {
		name       string
		providerID string
		pkgSpec    string
		want       string
	}{
		{
			name:       "with slash",
			providerID: "github",
			pkgSpec:    "denoland/deno",
			want:       "github-denoland-deno",
		},
		{
			name:       "with colon",
			providerID: "http",
			pkgSpec:    "example.com:8080/path",
			want:       "http-example.com-8080-path",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ToDir(tt.providerID, tt.pkgSpec); got != tt.want {
				t.Errorf("ToDir() = %v, want %v", got, tt.want)
			}
		})
	}
}
