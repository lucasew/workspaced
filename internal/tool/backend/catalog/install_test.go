package catalog_test

import (
	"context"
	"os"
	"strings"
	"testing"

	// Minimal driver set for tool install (not full prelude: that is reserved
	// for cmd/workspaced/root.go). fetchurl + httpclient download; exec extracts.
	"workspaced/internal/tool/backend"
	"workspaced/internal/tool/backend/catalog"
	_ "workspaced/internal/tool/backend/catalog/applications"
	_ "workspaced/internal/tool/backend/github"
	"workspaced/internal/tool/checks"
	_ "workspaced/pkg/driver/exec/native"
	_ "workspaced/pkg/driver/fetchurl/fetchurl"
	_ "workspaced/pkg/driver/httpclient/native"
	"workspaced/pkg/logging"
	"workspaced/pkg/taskgroup"
)

// isReleaseCI is true when running under CI on a git tag ref (release builds).
func isReleaseCI() bool {
	if os.Getenv("CI") == "" {
		return false
	}
	ref := os.Getenv("GITHUB_REF")
	return strings.HasPrefix(ref, "refs/tags/")
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

	for _, name := range catalog.ListTools() {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			tool, err := catalog.NewTool(name)
			if err != nil {
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
				t.Fatalf("ListVersions: %v", err)
			}
			if len(versions) == 0 {
				t.Fatal("ListVersions returned no versions")
			}

			dest := t.TempDir()
			if err := tool.Install(ctx, versions[0], dest); err != nil {
				t.Fatalf("Install(%q): %v", versions[0], err)
			}
			if fixer, ok := tool.(backend.InstallFixer); ok {
				if err := fixer.Fix(ctx, dest); err != nil {
					t.Fatalf("Fix: %v", err)
				}
			}
			if err := checks.Run(ctx, dest, tool); err != nil {
				t.Fatalf("checks.Run: %v", err)
			}
		})
	}
}
