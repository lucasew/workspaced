package apps

import (
	"slices"
	"testing"

	"workspaced/internal/modfile"
	"workspaced/internal/tool/backend"
)

func TestBiomeListVersionsFiltersMonorepoTags(t *testing.T) {
	t.Parallel()

	tool := &biomeTool{inner: stubTool{versions: []string{
		"@biomejs/js-api@6.0.0",
		"@biomejs/biome@2.5.0",
		"@biomejs/js-api@5.9.0",
		"@biomejs/biome@2.4.16",
		"@biomejs/biome@2.4.16-beta.1",
		"v1.9.4", // legacy, not the monorepo CLI tag
	}}}

	got, err := tool.ListVersions(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"2.5.0", "2.4.16"}
	if !slices.Equal(got, want) {
		t.Fatalf("ListVersions() = %v, want %v", got, want)
	}
}

func TestBiomeVersionFromTag(t *testing.T) {
	t.Parallel()

	cases := []struct {
		tag string
		ver string
		ok  bool
	}{
		{"@biomejs/biome@2.5.0", "2.5.0", true},
		{"@biomejs/js-api@6.0.0", "", false},
		{"@biomejs/biome@2.5.0-rc.1", "", false},
		{"2.5.0", "", false},
		{"", "", false},
	}
	for _, tc := range cases {
		ver, ok := biomeVersionFromTag(tc.tag)
		if ok != tc.ok || ver != tc.ver {
			t.Fatalf("biomeVersionFromTag(%q) = (%q, %v), want (%q, %v)", tc.tag, ver, ok, tc.ver, tc.ok)
		}
	}
}

func TestBiomeTagForVersion(t *testing.T) {
	t.Parallel()

	if got := biomeTagForVersion("2.5.0"); got != "@biomejs/biome@2.5.0" {
		t.Fatalf("biomeTagForVersion(2.5.0) = %q", got)
	}
	if got := biomeTagForVersion("@biomejs/biome@2.5.0"); got != "@biomejs/biome@2.5.0" {
		t.Fatalf("biomeTagForVersion(full tag) = %q", got)
	}
}

func TestParseBiomeAssetURL(t *testing.T) {
	t.Parallel()

	cases := []struct {
		url  string
		os   string
		arch string
		ok   bool
	}{
		{"https://example/biome-darwin-arm64", "darwin", "arm64", true},
		{"https://example/biome-linux-x64", "linux", "amd64", true},
		{"https://example/biome-linux-x64-musl", "linux", "amd64", true},
		{"https://example/biome-win32-x64.exe", "windows", "amd64", true},
		{"https://example/biome-win32-arm64.exe", "windows", "arm64", true},
		{"https://example/checksums.txt", "", "", false},
	}
	for _, tc := range cases {
		osName, arch, ok := parseBiomeAssetURL(tc.url)
		if ok != tc.ok || osName != tc.os || arch != tc.arch {
			t.Fatalf("parseBiomeAssetURL(%q) = (%q, %q, %v), want (%q, %q, %v)",
				tc.url, osName, arch, ok, tc.os, tc.arch, tc.ok)
		}
	}
}

func TestSelectBiomeArtifactPrefersNonMusl(t *testing.T) {
	t.Parallel()

	arts := []backend.Artifact{
		{OS: "linux", Arch: "amd64", URL: "https://example/biome-linux-x64-musl"},
		{OS: "linux", Arch: "amd64", URL: "https://example/biome-linux-x64"},
		{OS: "darwin", Arch: "arm64", URL: "https://example/biome-darwin-arm64"},
	}
	got := selectBiomeArtifact(arts, "linux", "amd64")
	if got == nil {
		t.Fatal("expected artifact")
	}
	if !stringsHasSuffix(got.URL, "biome-linux-x64") {
		t.Fatalf("selected %q, want non-musl linux x64", got.URL)
	}
}

func TestBiomeEnrichLockfileExtractVersion(t *testing.T) {
	t.Parallel()

	tool := &biomeTool{inner: stubTool{}}
	entry := &modfile.RenovateDependency{Ref: "registry:biome", CurrentValue: "2.5.0"}
	tool.EnrichLockfile(entry)
	if entry.Versioning != "semver" {
		t.Fatalf("Versioning = %q", entry.Versioning)
	}
	if entry.ExtractVersion == "" {
		t.Fatal("expected ExtractVersion for monorepo tags")
	}
}

func stringsHasSuffix(s, suf string) bool {
	return len(s) >= len(suf) && s[len(s)-len(suf):] == suf
}
