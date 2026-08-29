package apps

import (
	"runtime"
	"strings"
	"testing"

	_ "github.com/lucasew/workspaced/pkg/driver/httpclient/native"

	"github.com/lucasew/workspaced/pkg/logging"
)

func TestGoListArtifactsAcceptsVersionWithoutGoPrefix(t *testing.T) {
	t.Parallel()

	tool := &goTool{}
	ctx := logging.NewWriterContext(t.Output())

	versions, err := tool.ListVersions(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(versions) == 0 {
		t.Fatal("ListVersions returned no versions for this platform")
	}

	// Use a concrete version returned by our own ListVersions (guaranteed to have
	// had at least one archive entry for the current os/arch in the filter).
	artifacts, err := tool.ListArtifacts(ctx, versions[0])
	if err != nil {
		t.Fatalf("ListArtifacts(%q) failed: %v", versions[0], err)
	}
	if len(artifacts) != 1 {
		t.Fatalf("artifact count = %d, want 1", len(artifacts))
	}

	osName := runtime.GOOS
	archName := runtime.GOARCH
	// The URL must contain the version we asked for (with "go" prefix in the filename) and the platform.
	if !strings.Contains(artifacts[0].URL, "go"+versions[0]) {
		t.Fatalf("artifact URL = %q, expected to contain go%s", artifacts[0].URL, versions[0])
	}
	if !strings.Contains(artifacts[0].URL, osName+"-"+archName) && !strings.Contains(artifacts[0].URL, osName+"-"+archName+".") {
		t.Fatalf("artifact URL = %q, expected to reference %s-%s", artifacts[0].URL, osName, archName)
	}
	if !strings.HasPrefix(artifacts[0].Hash, "sha256:") {
		t.Fatalf("expected sha256 hash, got %q", artifacts[0].Hash)
	}
}

func TestGoListArtifactsAcceptsGoPrefixedVersion(t *testing.T) {
	t.Parallel()

	tool := &goTool{}
	ctx := logging.NewWriterContext(t.Output())

	versions, err := tool.ListVersions(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(versions) == 0 {
		t.Fatal("ListVersions returned no versions")
	}

	// Pass a "go"-prefixed version; the implementation should normalize it.
	prefixed := "go" + versions[0]
	artifacts, err := tool.ListArtifacts(ctx, prefixed)
	if err != nil {
		t.Fatalf("ListArtifacts(%q) failed: %v", prefixed, err)
	}
	if len(artifacts) != 1 {
		t.Fatalf("artifact count = %d, want 1", len(artifacts))
	}

	if !strings.Contains(artifacts[0].URL, "go"+versions[0]) {
		t.Fatalf("artifact URL = %q, expected to contain go%s", artifacts[0].URL, versions[0])
	}
}
