package tool

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"workspaced/pkg/logging"
	parsespec "workspaced/pkg/parse/spec"
	"workspaced/pkg/taskgroup"
	"workspaced/pkg/tool/backend"
)

// Manager orchestrates the lifecycle, installation, and storage mapping for external tools.
// It maps abstract tool specs (e.g., "github:cli/cli@v2.0.0") to concrete local directories,
// delegating artifact fetching to registered backend providers.
type Manager struct {
	toolsDir string
}

// NewManager initializes a tool manager, determining the localized root directory
// where all tool artifacts will be stored. Returns an error if the path cannot be resolved.
func NewManager() (*Manager, error) {
	toolsDir, err := GetToolsDir()
	if err != nil {
		return nil, err
	}
	return &Manager{
		toolsDir: toolsDir,
	}, nil
}

// Install parses the tool specification and persists the tool to the localized directory.
// It fetches the artifact via the underlying provider, resolving "latest" versions against
// upstream registries if needed.
func (m *Manager) Install(ctx context.Context, toolSpecStr string) error {
	return m.installWithHint(ctx, toolSpecStr, "")
}

// installWithHint executes the core installation flow, optionally taking a binaryHint
// (such as an expected executable name). When a hint is provided, it attempts an optimized
// artifact selection (via ArtifactTool) before falling back to standard backend.Tool logic.
// The hint helps disambiguate platforms where an archive might contain multiple binaries.
func (m *Manager) installWithHint(ctx context.Context, toolSpecStr string, binaryHint string) error {
	logger := logging.GetLogger(ctx)
	logger.Debug("installing tool", "input", toolSpecStr)
	spec, err := parsespec.Parse(toolSpecStr)
	if err != nil {
		return err
	}
	logger.Debug("parsed spec", "spec", spec)

	p, err := Get(spec.Provider)
	if err != nil {
		return err
	}

	t, err := p.Tool(spec.Package)
	if err != nil {
		return err
	}

	// Resolve latest version if needed
	version := spec.Version
	if version == "latest" {
		logger.Debug("resolving latest version", "pkg", spec.Package)
		versions, err := t.ListVersions(ctx)
		if err != nil {
			return fmt.Errorf("failed to list versions: %w", err)
		}
		if len(versions) == 0 {
			return fmt.Errorf("no versions found for package %s", spec.Package)
		}
		version = versions[0]
		logger.Debug("resolved latest version", "version", version)
	}

	normalizedVersion := normalizeVersion(version)
	logger.Debug("normalized version", "original", version, "normalized", normalizedVersion)

	destPath := filepath.Join(m.toolsDir, spec.Dir(), normalizedVersion)

	if err := os.MkdirAll(destPath, 0755); err != nil {
		return err
	}

	// The actual installation work (network + extract) can be long-running.
	// When a taskgroup.Group is present in the context we schedule it as a
	// child task under the Internet pool so it gets its own named entry,
	// Status updates, and captured logs in the progress system.
	doInstall := func(cctx context.Context) error {
		// Prefer the rich ArtifactTool path when we have a binary hint (better artifact scoring).
		if binaryHint != "" {
			if at, ok := t.(backend.ArtifactTool); ok {
				artifacts, err := at.ListArtifacts(cctx, version)
				if err == nil {
					if chosen := backend.SelectArtifact(artifacts, runtime.GOOS, runtime.GOARCH, binaryHint); chosen != nil {
						logger.Debug("installing with artifact hint", "url", chosen.URL, "hint", binaryHint)
						return at.InstallArtifact(cctx, *chosen, destPath)
					}
				}
			}
		}

		// Normal path: let the Tool do the install (it will select a suitable artifact for the platform).
		logger.Debug("installing tool via Tool.Install", "dest", destPath)
		return t.Install(cctx, version, destPath)
	}

	if parent := taskgroup.FromContext(ctx); parent != nil {
		child, _ := parent.SubGroup(ctx)
		var installErr error
		child.Go("install:"+spec.String(), taskgroup.Internet, func(cc context.Context, s *taskgroup.Status) error {
			s.Update("installing " + normalizedVersion)
			s.Progress(0, 1)
			installErr = doInstall(cc)
			s.Progress(1, 1)
			return installErr
		})
		if werr := child.Wait(); werr != nil && installErr == nil {
			installErr = werr
		}
		if installErr != nil {
			return fmt.Errorf("installation failed: %w", installErr)
		}
	} else {
		if err := doInstall(ctx); err != nil {
			return fmt.Errorf("installation failed: %w", err)
		}
	}

	logger.Info("tool installed successfully", "spec", spec, "normalized_version", normalizedVersion, "path", destPath)

	// TODO: Shell integration will handle PATH management
	// Shim generation removed - see shell hooks for dynamic PATH injection

	return nil
}

// InstalledTool represents a discrete version of a tool that has been physically
// persisted to the local system by the Manager.
type InstalledTool struct {
	Name    string
	Version string
	Path    string
}

// ListInstalled scans the localized tools directory and returns all present tool versions.
// This is an offline operation based on directory structure, and may include versions
// installed directly without a lockfile.
func (m *Manager) ListInstalled() ([]InstalledTool, error) {
	var tools []InstalledTool

	entries, err := os.ReadDir(m.toolsDir)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		toolName := entry.Name() // e.g., github-denoland-deno
		toolPath := filepath.Join(m.toolsDir, toolName)

		versions, err := os.ReadDir(toolPath)
		if err != nil {
			continue
		}

		for _, v := range versions {
			if !v.IsDir() {
				continue
			}
			tools = append(tools, InstalledTool{
				Name:    toolName,
				Version: v.Name(),
				Path:    filepath.Join(toolPath, v.Name()),
			})
		}
	}

	return tools, nil
}

// normalizeVersion removes the 'v' prefix from versions for consistent storage
func normalizeVersion(version string) string {
	version = strings.TrimPrefix(version, "v")
	// Replace slashes with dashes to avoid nested directories
	return strings.ReplaceAll(version, "/", "-")
}
