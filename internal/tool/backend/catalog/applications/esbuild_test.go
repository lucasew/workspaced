package apps

import (
	"strings"
	"testing"

	"workspaced/internal/modfile"
	_ "workspaced/pkg/driver/httpclient/native"
	"workspaced/pkg/logging"
)

func TestEsbuildPlatform(t *testing.T) {
	t.Parallel()

	cases := []struct {
		goos, goarch, want string
		ok                 bool
	}{
		{"linux", "amd64", "linux-x64", true},
		{"linux", "arm64", "linux-arm64", true},
		{"darwin", "amd64", "darwin-x64", true},
		{"darwin", "arm64", "darwin-arm64", true},
		{"windows", "amd64", "win32-x64", true},
		{"windows", "arm64", "win32-arm64", true},
		{"windows", "386", "win32-ia32", true},
		{"linux", "386", "linux-ia32", true},
		{"plan9", "amd64", "", false},
		{"linux", "sparc64", "", false},
	}
	for _, tc := range cases {
		got, ok := esbuildPlatform(tc.goos, tc.goarch)
		if ok != tc.ok || got != tc.want {
			t.Errorf("esbuildPlatform(%q, %q) = %q, %v; want %q, %v",
				tc.goos, tc.goarch, got, ok, tc.want, tc.ok)
		}
	}
}

func TestEsbuildArtifactURL(t *testing.T) {
	t.Parallel()

	got := esbuildArtifactURL("linux-x64", "0.28.1")
	want := "https://registry.npmjs.org/@esbuild/linux-x64/-/linux-x64-0.28.1.tgz"
	if got != want {
		t.Fatalf("esbuildArtifactURL = %q, want %q", got, want)
	}
}

func TestNormalizeEsbuildVersion(t *testing.T) {
	t.Parallel()

	cases := []struct{ in, want string }{
		{"0.28.1", "0.28.1"},
		{"v0.28.1", "0.28.1"},
		{" latest ", "latest"},
		{"", ""},
	}
	for _, tc := range cases {
		if got := normalizeEsbuildVersion(tc.in); got != tc.want {
			t.Errorf("normalizeEsbuildVersion(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

func TestEsbuildListArtifacts(t *testing.T) {
	t.Parallel()

	tool := &esbuildTool{}
	ctx := logging.NewWriterContext(t.Output())

	arts, err := tool.ListArtifacts(ctx, "0.28.1")
	if err != nil {
		t.Fatal(err)
	}
	if len(arts) != 1 {
		t.Fatalf("artifact count = %d, want 1", len(arts))
	}
	if !strings.Contains(arts[0].URL, "@esbuild/") {
		t.Fatalf("URL = %q, want npm @esbuild platform package", arts[0].URL)
	}
	if !strings.HasSuffix(arts[0].URL, "-0.28.1.tgz") {
		t.Fatalf("URL = %q, want versioned .tgz", arts[0].URL)
	}
	if arts[0].Hash != "" && !strings.HasPrefix(arts[0].Hash, "sha1:") {
		t.Fatalf("hash = %q, want empty or sha1:", arts[0].Hash)
	}
	if arts[0].Hash == "" {
		t.Log("warning: no tarball hash from npm registry (non-fatal)")
	}
}

func TestEsbuildListArtifactsAcceptsVPrefix(t *testing.T) {
	t.Parallel()

	tool := &esbuildTool{}
	ctx := logging.NewWriterContext(t.Output())

	arts, err := tool.ListArtifacts(ctx, "v0.28.1")
	if err != nil {
		t.Fatal(err)
	}
	if len(arts) != 1 {
		t.Fatalf("artifact count = %d, want 1", len(arts))
	}
	if !strings.HasSuffix(arts[0].URL, "-0.28.1.tgz") {
		t.Fatalf("URL = %q, expected v-prefix stripped", arts[0].URL)
	}
}

func TestEsbuildEnrichLockfile(t *testing.T) {
	t.Parallel()

	tool := &esbuildTool{}
	entry := &modfile.RenovateDependency{}
	tool.EnrichLockfile(entry)
	if entry.DepName != "esbuild" || entry.Datasource != "npm" || entry.Versioning != "semver" {
		t.Fatalf("EnrichLockfile = %+v", entry)
	}
}

func TestEsbuildListVersions(t *testing.T) {
	t.Parallel()

	tool := &esbuildTool{}
	ctx := logging.NewWriterContext(t.Output())

	vers, err := tool.ListVersions(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(vers) == 0 {
		t.Fatal("ListVersions returned no versions")
	}
	if strings.Contains(vers[0], "-") {
		t.Fatalf("first version %q should be stable (no prerelease suffix)", vers[0])
	}
	for _, v := range vers {
		if strings.Contains(v, "-") {
			t.Fatalf("prerelease leaked into ListVersions: %q", v)
		}
	}
}
