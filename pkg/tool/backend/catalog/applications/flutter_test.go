package apps

import (
	"log/slog"
	"runtime"
	"strings"
	"testing"

	_ "workspaced/pkg/driver/httpclient/native"

	"workspaced/pkg/logging"
)

func TestFlutterListVersionsAndArtifacts(t *testing.T) {
	t.Parallel()

	tool := &flutterTool{}
	ctx := logging.ContextWithLogger(t.Context(), slog.Default())

	versions, err := tool.ListVersions(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(versions) == 0 {
		t.Fatal("ListVersions returned no versions for this platform")
	}

	// Pick a concrete version we know exists for the platform.
	artifacts, err := tool.ListArtifacts(ctx, versions[0])
	if err != nil {
		t.Fatalf("ListArtifacts(%q) failed: %v", versions[0], err)
	}
	if len(artifacts) != 1 {
		t.Fatalf("artifact count = %d, want 1", len(artifacts))
	}

	a := artifacts[0]
	if a.URL == "" {
		t.Fatal("artifact URL is empty")
	}
	if !strings.HasPrefix(a.Hash, "sha256:") {
		t.Fatalf("expected sha256 hash, got %q", a.Hash)
	}

	// URL should point at Google storage and contain a flutter_ platform archive for the version.
	if !strings.Contains(a.URL, "flutter_infra_release/releases") {
		t.Fatalf("artifact URL = %q, expected to reference flutter storage", a.URL)
	}
	if !strings.Contains(a.URL, versions[0]) {
		t.Fatalf("artifact URL = %q, expected to contain version %s", a.URL, versions[0])
	}

	// Basic platform sanity in the filename for common cases.
	osName := runtime.GOOS
	switch osName {
	case "darwin":
		if runtime.GOARCH == "arm64" {
			if !strings.Contains(a.URL, "macos_arm64_") {
				t.Fatalf("artifact URL = %q, expected arm64 mac filename", a.URL)
			}
		} else {
			if !strings.Contains(a.URL, "flutter_macos_") || strings.Contains(a.URL, "arm64") {
				t.Fatalf("artifact URL = %q, expected macos (non-arm) filename", a.URL)
			}
		}
	case "linux":
		if !strings.Contains(a.URL, "flutter_linux_") {
			t.Fatalf("artifact URL = %q, expected linux filename", a.URL)
		}
	case "windows":
		if !strings.Contains(a.URL, "flutter_windows_") {
			t.Fatalf("artifact URL = %q, expected windows filename", a.URL)
		}
	}
}

func TestFlutterNormalizeAndLatest(t *testing.T) {
	t.Parallel()

	tool := &flutterTool{}
	ctx := logging.ContextWithLogger(t.Context(), slog.Default())

	// "latest" should resolve without error and produce an artifact.
	artifacts, err := tool.ListArtifacts(ctx, "latest")
	if err != nil {
		t.Fatalf("ListArtifacts(latest) failed: %v", err)
	}
	if len(artifacts) != 1 {
		t.Fatalf("latest artifact count = %d, want 1", len(artifacts))
	}

	// v-prefixed should be accepted.
	versions, _ := tool.ListVersions(ctx)
	if len(versions) > 0 {
		vpref := "v" + versions[0]
		arts2, err := tool.ListArtifacts(ctx, vpref)
		if err != nil {
			t.Fatalf("ListArtifacts(%q) failed: %v", vpref, err)
		}
		if len(arts2) != 1 {
			t.Fatalf("vprefixed artifact count = %d, want 1", len(arts2))
		}
	}
}
