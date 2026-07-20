package version

import (
	"runtime"
	"strings"
	"testing"
)

func TestPlatform(t *testing.T) {
	got := Platform()
	wantPrefix := runtime.GOOS + "-" + runtime.GOARCH
	if got != wantPrefix && !strings.HasPrefix(got, wantPrefix+"-") {
		t.Fatalf("Platform() = %q, want %q or %q-<microarch>", got, wantPrefix, wantPrefix)
	}
	// No spaces: separate token for --version.
	if strings.Contains(got, " ") {
		t.Fatalf("Platform() must be a single token, got %q", got)
	}
}

func TestVersionStringHasSpaceSeparatedPlatform(t *testing.T) {
	got := VersionString()
	parts := strings.Split(got, " ")
	if len(parts) != 2 {
		t.Fatalf("VersionString() = %q, want exactly two space-separated tokens", got)
	}
	if parts[0] != GetBuildID() {
		t.Fatalf("first token = %q, want GetBuildID() %q", parts[0], GetBuildID())
	}
	if parts[1] != Platform() {
		t.Fatalf("second token = %q, want Platform() %q", parts[1], Platform())
	}
}

func TestGetBuildIDNoSpace(t *testing.T) {
	if strings.Contains(GetBuildID(), " ") {
		t.Fatalf("GetBuildID() must stay filename-safe, got %q", GetBuildID())
	}
}
