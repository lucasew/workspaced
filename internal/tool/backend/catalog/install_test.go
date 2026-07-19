package catalog_test

import (
	"context"
	"errors"
	"fmt"
	"os"
	"runtime"
	"strings"
	"sync"
	"testing"

	// Minimal driver set for tool install (not full prelude: that is reserved
	// for cmd/workspaced/root.go). fetchurl + httpclient download; exec extracts.
	"github.com/lucasew/workspaced/internal/tool/backend"
	"github.com/lucasew/workspaced/internal/tool/backend/catalog"
	apps "github.com/lucasew/workspaced/internal/tool/backend/catalog/applications"
	"github.com/lucasew/workspaced/internal/tool/backend/github"
	"github.com/lucasew/workspaced/internal/tool/checks"
	_ "github.com/lucasew/workspaced/pkg/driver/exec/native"
	_ "github.com/lucasew/workspaced/pkg/driver/fetchurl/fetchurl"
	_ "github.com/lucasew/workspaced/pkg/driver/httpclient/native"
	"github.com/lucasew/workspaced/pkg/logging"
	"github.com/lucasew/workspaced/pkg/taskgroup"
)

// isReleaseCI is true when running under CI on a git tag ref (release builds).
func isReleaseCI() bool {
	if os.Getenv("CI") == "" {
		return false
	}
	ref := os.Getenv("GITHUB_REF")
	return strings.HasPrefix(ref, "refs/tags/")
}

// stepSummaryMu serializes appends to GITHUB_STEP_SUMMARY (parallel subtests).
var stepSummaryMu sync.Mutex

func appendStepSummary(s string) {
	path := os.Getenv("GITHUB_STEP_SUMMARY")
	if path == "" {
		return
	}
	stepSummaryMu.Lock()
	defer stepSummaryMu.Unlock()
	f, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return
	}
	defer f.Close()
	_, _ = f.WriteString(s)
}

func reportInstallFailure(tool, msg string) {
	appendStepSummary(fmt.Sprintf("### `%s`\n\n```\n%s\n```\n\n", tool, strings.TrimSpace(msg)))
}

func testInstallContext(t *testing.T) (ctx context.Context, wait func()) {
	t.Helper()
	base := logging.NewWriterContext(t.Output())
	g, ctx := taskgroup.New(base, taskgroup.DefaultLimits())
	return ctx, func() {
		// Best-effort drain. Failed fetchurl attempts can leave canceled
		// Internet tasks that would otherwise block Wait until the test timeout.
		done := make(chan error, 1)
		go func() { done <- g.Wait() }()
		select {
		case err := <-done:
			if err != nil && !t.Failed() {
				t.Errorf("taskgroup: %v", err)
			}
		case <-t.Context().Done():
		}
	}
}

func TestRegistryInstallChecksDeclared(t *testing.T) {
	t.Parallel()
	for _, name := range catalog.ListTools() {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			tool, err := catalog.NewTool(name)
			if err != nil {
				t.Fatalf("NewTool(%q): %v", name, err)
			}
			ic, ok := tool.(checks.InstallChecker)
			if !ok {
				return
			}
			if len(ic.InstallChecks()) == 0 {
				t.Fatalf("%q implements InstallChecker but InstallChecks() is empty", name)
			}
		})
	}
}

func TestRegistryInstall(t *testing.T) {
	// Opt-in locally via WORKSPACED_TEST_TOOL_INSTALL=1.
	// On CI, also runs automatically for release tags (refs/tags/*).
	if os.Getenv("WORKSPACED_TEST_TOOL_INSTALL") != "1" && !isReleaseCI() {
		t.Skip("set WORKSPACED_TEST_TOOL_INSTALL=1 (or run on a release tag in CI) to run registry install checks")
	}

	target := os.Getenv("TARGET")
	if target == "" {
		target = runtime.GOOS + "/" + runtime.GOARCH
	}
	appendStepSummary(fmt.Sprintf("## Registry install (`%s`)\n\n", target))

	for _, name := range catalog.ListTools() {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			tool, err := catalog.NewTool(name)
			if err != nil {
				reportInstallFailure(name, fmt.Sprintf("NewTool: %v", err))
				t.Fatalf("NewTool(%q): %v", name, err)
			}
			ic, ok := tool.(checks.InstallChecker)
			if !ok || len(ic.InstallChecks()) == 0 {
				t.Skip("no install checks")
			}

			ctx, wait := testInstallContext(t)
			defer wait()

			versions, err := tool.ListVersions(ctx)
			if err != nil {
				reportInstallFailure(name, fmt.Sprintf("ListVersions: %v", err))
				t.Fatalf("ListVersions: %v", err)
			}
			if len(versions) == 0 {
				reportInstallFailure(name, "ListVersions returned no versions")
				t.Fatal("ListVersions returned no versions")
			}

			dest := t.TempDir()
			if err := tool.Install(ctx, versions[0], dest); err != nil {
				// Upstream may not ship binaries for every GOOS/GOARCH (e.g.
				// resvg has no linux/arm64 release). That is not a registry bug.
				if errors.Is(err, github.ErrNoArtifact) || errors.Is(err, apps.ErrNoPlatformArtifact) {
					t.Skipf("no artifact for %s/%s: %v", runtime.GOOS, runtime.GOARCH, err)
				}
				reportInstallFailure(name, fmt.Sprintf("Install(%q): %v", versions[0], err))
				t.Fatalf("Install(%q): %v", versions[0], err)
			}
			if fixer, ok := tool.(backend.InstallFixer); ok {
				if err := fixer.Fix(ctx, dest); err != nil {
					reportInstallFailure(name, fmt.Sprintf("Fix: %v", err))
					t.Fatalf("Fix: %v", err)
				}
			}
			if err := checks.Run(ctx, dest, tool); err != nil {
				reportInstallFailure(name, fmt.Sprintf("checks.Run: %v", err))
				t.Fatalf("checks.Run: %v", err)
			}
		})
	}
}
