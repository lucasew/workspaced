package tool

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	parsespec "workspaced/pkg/parse/spec"
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
	return m.installWithHint(ctx, toolSpecStr, "")
}

func (m *Manager) installWithHint(ctx context.Context, toolSpecStr string, binaryHint string) error {
	slog.Debug("installing tool", "input", toolSpecStr)
	spec, err := parsespec.Parse(toolSpecStr)
	if err != nil {
		return err
	}
	slog.Debug("parsed spec", "spec", spec)

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
		slog.Debug("resolving latest version", "pkg", spec.Package)
		versions, err := t.ListVersions(ctx)
		if err != nil {
			return fmt.Errorf("failed to list versions: %w", err)
		}
		if len(versions) == 0 {
			return fmt.Errorf("no versions found for package %s", spec.Package)
		}
		version = versions[0]
		slog.Debug("resolved latest version", "version", version)
	}

	normalizedVersion := normalizeVersion(version)
	slog.Debug("normalized version", "original", version, "normalized", normalizedVersion)

	destPath := filepath.Join(m.toolsDir, spec.Dir(), normalizedVersion)

	if err := os.MkdirAll(destPath, 0755); err != nil {
		return err
	}

	// Prefer the rich ArtifactTool path when we have a binary hint (better artifact scoring).
	if binaryHint != "" {
		if at, ok := t.(provider.ArtifactTool); ok {
			artifacts, err := at.ListArtifacts(ctx, version)
			if err == nil {
				if chosen := provider.SelectArtifact(artifacts, runtime.GOOS, runtime.GOARCH, binaryHint); chosen != nil {
					slog.Debug("installing with artifact hint", "url", chosen.URL, "hint", binaryHint)
					if err := at.InstallArtifact(ctx, *chosen, destPath); err != nil {
						return fmt.Errorf("installation failed: %w", err)
					}
					slog.Info("tool installed successfully", "spec", spec, "normalized_version", normalizedVersion, "path", destPath)
					return nil
				}
			}
		}
	}

	// Normal path: let the Tool do the install (it will select a suitable artifact for the platform).
	slog.Debug("installing tool via Tool.Install", "dest", destPath)
	if err := t.Install(ctx, version, destPath); err != nil {
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

// normalizeVersion removes the 'v' prefix from versions for consistent storage
func normalizeVersion(version string) string {
	version = strings.TrimPrefix(version, "v")
	// Replace slashes with dashes to avoid nested directories
	return strings.ReplaceAll(version, "/", "-")
}
