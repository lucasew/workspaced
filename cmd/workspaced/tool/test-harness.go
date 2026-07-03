package tool

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"runtime"

	"workspaced/pkg/logging"
	parsespec "workspaced/pkg/parse/spec"
	"workspaced/pkg/taskgroup"
	"workspaced/pkg/tool"
	"workspaced/pkg/tool/backend"

	"github.com/spf13/cobra"
)

func init() {
	Registry.Register(func(c *cobra.Command) {
		c.AddCommand(&cobra.Command{
			Use:   "test-harness <tool-spec> [expected-bin-path] [expected-version]",
			Short: "Test tool installation and version assumptions",
			Long: `Install a tool in a temporary location and verify it behaves as expected.
Only runs if the WORKSPACED_RUN_TEST_HARNESS environment variable is set to "1" or "true".`,
			Args: cobra.MinimumNArgs(1),
			RunE: func(cmd *cobra.Command, args []string) error {
				if os.Getenv("WORKSPACED_RUN_TEST_HARNESS") != "1" && os.Getenv("WORKSPACED_RUN_TEST_HARNESS") != "true" {
					cmd.Println("Skipping: WORKSPACED_RUN_TEST_HARNESS is not set")
					return nil
				}

				specStr := args[0]
				var expectedBin, expectedVersion string
				if len(args) > 1 {
					expectedBin = args[1]
				}
				if len(args) > 2 {
					expectedVersion = args[2]
				}

				return runTestHarness(cmd.Context(), specStr, expectedBin, expectedVersion)
			},
		})
	})
}

func runTestHarness(ctx context.Context, specStr, expectedBin, expectedVersion string) error {
	logger := logging.GetLogger(ctx)
	logger.Info("running test harness", "spec", specStr)

	tempDir, err := os.MkdirTemp("", "workspaced-test-harness-*")
	if err != nil {
		return fmt.Errorf("failed to create temp dir: %w", err)
	}
	defer os.RemoveAll(tempDir)

	spec, err := parsespec.Parse(specStr)
	if err != nil {
		return fmt.Errorf("failed to parse spec: %w", err)
	}

	p, err := tool.Get(spec.Provider)
	if err != nil {
		return fmt.Errorf("failed to get provider: %w", err)
	}

	t, err := p.Tool(spec.Package)
	if err != nil {
		return fmt.Errorf("failed to get tool: %w", err)
	}

	actualVersion := spec.Version
	if actualVersion == "latest" {
		versions, err := t.ListVersions(ctx)
		if err != nil {
			return fmt.Errorf("failed to list versions: %w", err)
		}
		if len(versions) == 0 {
			return fmt.Errorf("no versions found")
		}
		actualVersion = versions[0]
	}

	if expectedVersion != "" && actualVersion != expectedVersion {
		return fmt.Errorf("version mismatch: expected %s, got %s", expectedVersion, actualVersion)
	}

	installDir := filepath.Join(tempDir, spec.Dir(), actualVersion)
	if err := os.MkdirAll(installDir, 0755); err != nil {
		return fmt.Errorf("failed to create install dir: %w", err)
	}

	doInstall := func(ctx context.Context) error {
		if expectedBin != "" {
			if at, ok := t.(backend.ArtifactTool); ok {
				artifacts, err := at.ListArtifacts(ctx, actualVersion)
				if err == nil {
					if chosen := backend.SelectArtifact(artifacts, runtime.GOOS, runtime.GOARCH, expectedBin); chosen != nil {
						return at.InstallArtifact(ctx, *chosen, installDir)
					}
				}
			}
		}
		return t.Install(ctx, actualVersion, installDir)
	}

	var installErr error
	if parent := taskgroup.FromContext(ctx); parent != nil {
		child, _ := parent.SubGroup(ctx)
		child.Go("install:"+spec.String(), taskgroup.Internet, func(ctx context.Context, s *taskgroup.Status) error {
			s.Update("installing")
			s.Progress(0, 1)
			installErr = doInstall(ctx)
			s.Progress(1, 1)
			return installErr
		})
		if werr := child.Wait(); werr != nil && installErr == nil {
			installErr = werr
		}
	} else {
		installErr = doInstall(ctx)
	}

	if installErr != nil {
		return fmt.Errorf("installation failed: %w", installErr)
	}

	if expectedBin != "" {
		binPath := tool.FindBinary(installDir, expectedBin)
		if binPath == "" {
			return fmt.Errorf("expected binary %s not found in %s", expectedBin, installDir)
		}
		logger.Info("found binary", "path", binPath)
	}

	logger.Info("test harness passed")
	return nil
}
