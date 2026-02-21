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

	artifact := findArtifact(artifacts, runtime.GOOS, runtime.GOARCH, binaryHint)
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

func findArtifact(artifacts []provider.Artifact, osName, arch string, binaryHint string) *provider.Artifact {
	var candidates []provider.Artifact
	for _, a := range artifacts {
		if a.OS == osName && a.Arch == arch {
			if strings.HasSuffix(a.URL, ".deb") {
				continue
			}
			if strings.HasSuffix(a.URL, ".rpm") {
				continue
			}
			candidates = append(candidates, a)
		}
	}

	if len(candidates) == 0 {
		return nil
	}

	hint := strings.ToLower(strings.TrimSpace(binaryHint))
	sort.Slice(candidates, func(i, j int) bool {
		si := scoreArtifactForHint(candidates[i].URL, hint)
		sj := scoreArtifactForHint(candidates[j].URL, hint)
		if si != sj {
			return si > sj
		}
		return len(candidates[i].URL) < len(candidates[j].URL)
	})

	return &candidates[0]
}

func scoreArtifactForHint(url string, hint string) int {
	if hint == "" {
		return 0
	}

	base := strings.ToLower(filepath.Base(url))
	score := 0

	// Strong matches for tokenized binary names (resvg-*, *_resvg_*, etc.)
	for _, sep := range []string{"-", "_", "."} {
		if strings.Contains(base, hint+sep) || strings.Contains(base, sep+hint+sep) || strings.Contains(base, sep+hint+".") {
			score += 120
			break
		}
	}

	// Generic match fallback
	if strings.Contains(base, hint) {
		score += 60
	}

	// Slightly prefer common distributable archives for CLI tools
	if strings.HasSuffix(base, ".tar.gz") || strings.HasSuffix(base, ".tgz") || strings.HasSuffix(base, ".zip") {
		score += 10
	}

	// Avoid obvious debug/minimal artifacts when possible
	if strings.Contains(base, "debug") {
		score -= 20
	}

	return score
}

// normalizeVersion removes the 'v' prefix from versions for consistent storage
func normalizeVersion(version string) string {
	version = strings.TrimPrefix(version, "v")
	// Replace slashes with dashes to avoid nested directories
	return strings.ReplaceAll(version, "/", "-")
}
