package tool

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"workspaced/pkg/tool/provider"
)

type Manager struct {
	toolsDir string
}

func NewManager() (*Manager, error) {
	toolsDir, err := GetToolsDir()
	if err != nil {
		return nil, err
	}
	return &Manager{
		toolsDir: toolsDir,
	}, nil
}

func (m *Manager) Install(ctx context.Context, toolSpecStr string) error {
	slog.Debug("installing tool", "input", toolSpecStr)
	spec, err := ParseToolSpec(toolSpecStr)
	if err != nil {
		return err
	}
	slog.Debug("parsed spec", "spec", spec)

	p, err := GetProvider(spec.Provider)
	if err != nil {
		return err
	}

	pkgConfig, err := p.ParsePackage(spec.Package)
	if err != nil {
		return err
	}

	// Resolve latest version if needed
	version := spec.Version
	if version == "latest" {
		slog.Debug("resolving latest version", "pkg", spec.Package)
		versions, err := p.ListVersions(ctx, pkgConfig)
		if err != nil {
			return fmt.Errorf("failed to list versions: %w", err)
		}
		if len(versions) == 0 {
			return fmt.Errorf("no versions found for package %s", spec.Package)
		}
		// Assuming ListVersions returns unsorted or sorted?
		// GitHub provider returns in API order (usually desc time).
		version = versions[0]
		slog.Debug("resolved latest version", "version", version)
	}

	slog.Debug("fetching artifacts", "version", version)
	artifacts, err := p.GetArtifacts(ctx, pkgConfig, version)
	if err != nil {
		return fmt.Errorf("failed to get artifacts: %w", err)
	}
	slog.Debug("found artifacts", "count", len(artifacts))

	artifact := findArtifact(artifacts, runtime.GOOS, runtime.GOARCH)
	if artifact == nil {
		return fmt.Errorf("no artifact found for platform %s/%s", runtime.GOOS, runtime.GOARCH)
	}
	slog.Debug("selected artifact", "url", artifact.URL, "os", artifact.OS, "arch", artifact.Arch)

	// Normalize version by removing 'v' prefix for storage
	normalizedVersion := normalizeVersion(version)
	slog.Debug("normalized version", "original", version, "normalized", normalizedVersion)

	destPath := filepath.Join(m.toolsDir, spec.Dir(), normalizedVersion)

	if err := os.MkdirAll(destPath, 0755); err != nil {
		return err
	}

	slog.Debug("installing artifact", "dest", destPath)
	if err := p.Install(ctx, *artifact, destPath); err != nil {
		return fmt.Errorf("installation failed: %w", err)
	}
	slog.Info("tool installed successfully", "spec", spec, "normalized_version", normalizedVersion, "path", destPath)

	// TODO: Shell integration will handle PATH management
	// Shim generation removed - see shell hooks for dynamic PATH injection

	return nil
}

type InstalledTool struct {
	Name    string
	Version string
	Path    string
}

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

func findArtifact(artifacts []provider.Artifact, osName, arch string) *provider.Artifact {
	var candidates []provider.Artifact
	for _, a := range artifacts {
		if a.OS == osName && a.Arch == arch {
			candidates = append(candidates, a)
		}
	}

	if len(candidates) == 0 {
		return nil
	}

	sort.Slice(candidates, func(i, j int) bool {
		return len(candidates[i].URL) < len(candidates[j].URL)
	})

	return &candidates[0]
}

// normalizeVersion removes the 'v' prefix from versions for consistent storage
func normalizeVersion(version string) string {
	if strings.HasPrefix(version, "v") {
		return version[1:]
	}
	return version
}
