package catalog_test

import (
	"os"
	"testing"

	_ "workspaced/pkg/driver/httpclient/native"
	"workspaced/pkg/logging"
	"workspaced/pkg/tool/backend"
	"workspaced/pkg/tool/backend/catalog"
	_ "workspaced/pkg/tool/backend/catalog/applications"
	_ "workspaced/pkg/tool/backend/github"
	"workspaced/pkg/tool/checks"
)

func TestRegistryInstallChecksDeclared(t *testing.T) {
	t.Parallel()
	for _, name := range catalog.ListTools() {
		name := name
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
	if os.Getenv("WORKSPACED_TEST_TOOL_INSTALL") != "1" {
		t.Skip("set WORKSPACED_TEST_TOOL_INSTALL=1 to run registry install checks")
	}

	for _, name := range catalog.ListTools() {
		name := name
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

			ctx := logging.NewRootContext(nil)
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
