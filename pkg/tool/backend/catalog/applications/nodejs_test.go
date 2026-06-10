package apps

import (
	"fmt"
	"log/slog"
	"runtime"
	"strings"
	"testing"

	"workspaced/pkg/logging"
)

func TestNodejsListArtifactsAcceptsVersionWithoutVPrefix(t *testing.T) {
	t.Parallel()

	tool := &nodejsTool{}
	artifacts, err := tool.ListArtifacts(logging.ContextWithLogger(t.Context(), slog.Default()), "22.16.0")
	if err != nil {
		t.Fatal(err)
	}
	if len(artifacts) != 1 {
		t.Fatalf("artifact count = %d, want 1", len(artifacts))
	}

	osPart, archPart, ext := tool.nodePlatformAndExt()
	wantFilename := fmt.Sprintf("node-v22.16.0-%s-%s%s", osPart, archPart, ext)
	wantURL := fmt.Sprintf("https://nodejs.org/dist/v22.16.0/%s", wantFilename)

	if artifacts[0].URL != wantURL {
		t.Fatalf("artifact URL = %q, want %q", artifacts[0].URL, wantURL)
	}
	if runtime.GOOS != "windows" && !strings.HasSuffix(artifacts[0].URL, ".tar.gz") {
		t.Fatalf("artifact URL = %q, want tarball URL", artifacts[0].URL)
	}
}
