package tool_test

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"workspaced/pkg/logging"
	parsespec "workspaced/pkg/parse/spec"
	"workspaced/pkg/taskgroup"
	"workspaced/pkg/tool"
	"workspaced/pkg/tool/backend"

	_ "workspaced/pkg/driver/exec/native"         // register exec
	_ "workspaced/pkg/driver/fetchurl/fetchurl" // register fetchurl
	_ "workspaced/pkg/driver/httpclient/native" // register httpclient
	_ "workspaced/pkg/tool/prelude"             // register backends
)

func TestToolRegistryHarness(t *testing.T) {
	if os.Getenv("WORKSPACED_RUN_TEST_HARNESS") != "1" && os.Getenv("WORKSPACED_RUN_TEST_HARNESS") != "true" {
		t.Skip("Skipping: WORKSPACED_RUN_TEST_HARNESS is not set")
	}

	testCases := []struct {
		specStr         string
		expectedBin     string
		expectedVersion string
	}{
		{specStr: "biome", expectedBin: "biome"},
		{specStr: "nodejs", expectedBin: "node"},
		{specStr: "golang", expectedBin: "go"},
	}

	for _, tc := range testCases {
		t.Run(tc.specStr, func(t *testing.T) {
			_, ctx := taskgroup.New(logging.NewRootContext(nil), taskgroup.DefaultLimits())

			tempDir, err := os.MkdirTemp("", "workspaced-test-harness-*")
			if err != nil {
				t.Fatalf("failed to create temp dir: %v", err)
			}
			defer os.RemoveAll(tempDir)

			spec, err := parsespec.Parse(tc.specStr)
			if err != nil {
				t.Fatalf("failed to parse spec: %v", err)
			}

			p, err := tool.Get(spec.Provider)
			if err != nil {
				t.Fatalf("failed to get provider: %v", err)
			}

			toolImpl, err := p.Tool(spec.Package)
			if err != nil {
				t.Fatalf("failed to get tool: %v", err)
			}

			actualVersion := spec.Version
			if actualVersion == "latest" {
				versions, err := toolImpl.ListVersions(ctx)
				if err != nil {
					t.Fatalf("failed to list versions: %v", err)
				}
				if len(versions) == 0 {
					t.Fatalf("no versions found")
				}
				actualVersion = versions[0]
			}

			if tc.expectedVersion != "" && actualVersion != tc.expectedVersion {
				t.Fatalf("version mismatch: expected %s, got %s", tc.expectedVersion, actualVersion)
			}

			installDir := filepath.Join(tempDir, spec.Dir(), actualVersion)
			if err := os.MkdirAll(installDir, 0755); err != nil {
				t.Fatalf("failed to create install dir: %v", err)
			}

			if tc.expectedBin != "" {
				if at, ok := toolImpl.(backend.ArtifactTool); ok {
					artifacts, err := at.ListArtifacts(ctx, actualVersion)
					if err == nil {
						if chosen := backend.SelectArtifact(artifacts, runtime.GOOS, runtime.GOARCH, tc.expectedBin); chosen != nil {
							err := at.InstallArtifact(ctx, *chosen, installDir)
							if err != nil {
								t.Fatalf("installation failed: %v", err)
							}
							goto Verify
						}
					}
				}
			}

			if err := toolImpl.Install(ctx, actualVersion, installDir); err != nil {
				t.Fatalf("installation failed: %v", err)
			}

		Verify:
			if tc.expectedBin != "" {
				binPath := tool.FindBinary(installDir, tc.expectedBin)
				if binPath == "" {
					t.Fatalf("expected binary %s not found in %s", tc.expectedBin, installDir)
				}
			}
		})
	}
}
