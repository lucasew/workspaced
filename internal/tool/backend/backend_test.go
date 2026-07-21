package backend

import (
	"path/filepath"
	"testing"
)

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

func TestSelectCodexCLIAsset(t *testing.T) {
	names := []string{
		"codex-x86_64-unknown-linux-musl.tar.gz",
		"codex-x86_64-unknown-linux-musl.zst",
		"codex-app-server-x86_64-unknown-linux-musl.tar.gz",
		"codex-package-x86_64-unknown-linux-musl.tar.gz",
		"codex-npm-linux-x64-0.142.5.tgz",
		"codex-zsh-x86_64-unknown-linux-musl.tar.gz",
		"codex-responses-api-proxy-x86_64-unknown-linux-musl.tar.gz",
	}
	arts := make([]Artifact, 0, len(names))
	for _, n := range names {
		arts = append(arts, Artifact{
			OS: "linux", Arch: "amd64",
			URL: "https://github.com/openai/codex/releases/download/rust-v0.142.5/" + n,
		})
	}
	got := SelectArtifact(arts, "linux", "amd64", "codex")
	if got == nil {
		t.Fatal("no artifact selected")
	}
	if base := filepath.Base(got.URL); base != "codex-x86_64-unknown-linux-musl.tar.gz" {
		t.Fatalf("SelectArtifact() = %s, want codex-x86_64-unknown-linux-musl.tar.gz", base)
	}
}

func TestSelectArtifactPrefersAndroidOverLinux(t *testing.T) {
	// Mirrors workspaced release asset names: Android and Linux both arm64.
	// Before the exact-OS bonus, equal scores + shorter-URL tiebreaker picked Linux.
	arts := []Artifact{
		{OS: "linux", Arch: "arm64", URL: "https://example.com/workspaced_Linux_arm64.tar.gz"},
		{OS: "android", Arch: "arm64", URL: "https://example.com/workspaced_Android_arm64.tar.gz"},
		{OS: "darwin", Arch: "arm64", URL: "https://example.com/workspaced_Darwin_arm64.tar.gz"},
	}
	got := SelectArtifact(arts, "android", "arm64", "workspaced")
	if got == nil {
		t.Fatal("no artifact selected")
	}
	if got.OS != "android" {
		t.Fatalf("SelectArtifact() OS = %s, want android (URL %s)", got.OS, got.URL)
	}
	if base := filepath.Base(got.URL); base != "workspaced_Android_arm64.tar.gz" {
		t.Fatalf("SelectArtifact() = %s, want workspaced_Android_arm64.tar.gz", base)
	}
}

func TestSelectArtifactAndroidFallsBackToLinux(t *testing.T) {
	arts := []Artifact{
		{OS: "linux", Arch: "arm64", URL: "https://example.com/tool_Linux_arm64.tar.gz"},
		{OS: "darwin", Arch: "arm64", URL: "https://example.com/tool_Darwin_arm64.tar.gz"},
	}
	got := SelectArtifact(arts, "android", "arm64", "tool")
	if got == nil {
		t.Fatal("no artifact selected")
	}
	if got.OS != "linux" {
		t.Fatalf("SelectArtifact() OS = %s, want linux fallback", got.OS)
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
			want: 511, // exact OS 500 + baseline 1 + 10 for archive
		},
		{
			name: "exact token match",
			art:  Artifact{OS: osName, Arch: arch, URL: "https://example.com/resvg-linux.tar.gz"},
			hint: "resvg",
			want: 690, // exact OS 500 + token 120 + substring 60 + archive 10
		},
		{
			name: "substring match",
			art:  Artifact{OS: osName, Arch: arch, URL: "https://example.com/myresvgtool.tar.gz"},
			hint: "resvg",
			want: 570, // exact OS 500 + substring 60 + archive 10
		},
		{
			name: "no name match but good archive",
			art:  Artifact{OS: osName, Arch: arch, URL: "https://example.com/other.tar.gz"},
			hint: "resvg",
			want: 510, // exact OS 500 + archive 10
		},
		{
			name: "token match + debug penalty",
			art:  Artifact{OS: osName, Arch: arch, URL: "https://example.com/resvg-debug.tar.gz"},
			hint: "resvg",
			want: 670, // exact OS 500 + token 120 + substring 60 + archive 10 - debug 20
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
		{
			name: "codex primary archive beats npm tarball",
			art:  Artifact{OS: osName, Arch: arch, URL: "https://example.com/codex-x86_64-unknown-linux-musl.tar.gz"},
			hint: "codex",
			want: 730, // exact OS 500 + token 120 + substring 60 + archive 10 + arch-token 40
		},
		{
			name: "codex npm package is penalized",
			art:  Artifact{OS: osName, Arch: arch, URL: "https://example.com/codex-npm-linux-x64-0.1.0.tgz"},
			hint: "codex",
			want: 640, // exact OS 500 + token 120 + substring 60 + archive 10 - npm 50
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

func TestScoreArtifactAndroidPrefersExactOverLinuxFallback(t *testing.T) {
	androidArt := Artifact{OS: "android", Arch: "arm64", URL: "https://example.com/workspaced_Android_arm64.tar.gz"}
	linuxArt := Artifact{OS: "linux", Arch: "arm64", URL: "https://example.com/workspaced_Linux_arm64.tar.gz"}

	androidScore := ScoreArtifact(androidArt, "android", "arm64", "workspaced")
	linuxScore := ScoreArtifact(linuxArt, "android", "arm64", "workspaced")
	if androidScore <= 0 {
		t.Fatalf("android artifact score = %d, want > 0", androidScore)
	}
	if linuxScore <= 0 {
		t.Fatalf("linux fallback score = %d, want > 0 (still eligible)", linuxScore)
	}
	if androidScore <= linuxScore {
		t.Fatalf("android score %d <= linux fallback %d; exact OS should win", androidScore, linuxScore)
	}
}
