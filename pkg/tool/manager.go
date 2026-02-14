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
	shimsDir string
}

func NewManager() (*Manager, error) {
	toolsDir, err := GetToolsDir()
	if err != nil {
		return nil, err
	}
	shimsDir, err := GetShimsDir()
	if err != nil {
		return nil, err
	}
	return &Manager{
		toolsDir: toolsDir,
		shimsDir: shimsDir,
	}, nil
}

func (m *Manager) Install(ctx context.Context, toolSpec string) error {
	slog.Debug("installing tool", "spec", toolSpec)
	providerID, pkgSpec, version, err := ParseToolSpec(toolSpec)
	if err != nil {
		return err
	}
	slog.Debug("parsed spec", "provider", providerID, "pkg", pkgSpec, "version", version)

	p, err := GetProvider(providerID)
	if err != nil {
		return err
	}

	pkgConfig, err := p.ParsePackage(pkgSpec)
	if err != nil {
		return err
	}

	// Resolve latest version if needed
	if version == "latest" {
		slog.Debug("resolving latest version", "pkg", pkgSpec)
		versions, err := p.ListVersions(ctx, pkgConfig)
		if err != nil {
			return fmt.Errorf("failed to list versions: %w", err)
		}
		if len(versions) == 0 {
			return fmt.Errorf("no versions found for package %s", pkgSpec)
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

	toolDirName := SpecToDir(providerID, pkgSpec)
	destPath := filepath.Join(m.toolsDir, toolDirName, normalizedVersion)

	if err := os.MkdirAll(destPath, 0755); err != nil {
		return err
	}

	slog.Debug("installing artifact", "dest", destPath)
	if err := p.Install(ctx, *artifact, destPath); err != nil {
		return fmt.Errorf("installation failed: %w", err)
	}
	slog.Debug("installation successful")

	// Shim generation
	binaries, err := findBinaries(destPath)
	if err != nil {
		return fmt.Errorf("failed to find binaries: %w", err)
	}
	slog.Debug("found binaries", "count", len(binaries), "binaries", binaries)

	if err := os.MkdirAll(m.shimsDir, 0755); err != nil {
		return err
	}

	for _, bin := range binaries {
		toolName := filepath.Base(bin)
		slog.Debug("generating shim", "tool", toolName, "target", bin)
		if err := GenerateShim(m.shimsDir, toolName); err != nil {
			return fmt.Errorf("failed to generate shim for %s: %w", toolName, err)
		}
	}

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

func findBinaries(dir string) ([]string, error) {
	var binaries []string

	// Check root
	rootBins, err := scanDirForBinaries(dir)
	if err != nil {
		return nil, err
	}
	binaries = append(binaries, rootBins...)

	// Check bin/ subdirectory
	binDir := filepath.Join(dir, "bin")
	if info, err := os.Stat(binDir); err == nil && info.IsDir() {
		subBins, err := scanDirForBinaries(binDir)
		if err != nil {
			return nil, err
		}
		binaries = append(binaries, subBins...)
	}

	return binaries, nil
}

func scanDirForBinaries(dir string) ([]string, error) {
	var binaries []string
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}

	for _, e := range entries {
		if e.IsDir() {
			continue
		}

		info, err := e.Info()
		if err != nil {
			continue
		}

		// Check executable bit
		if info.Mode()&0111 != 0 {
			binaries = append(binaries, filepath.Join(dir, e.Name()))
		} else if runtime.GOOS == "windows" && strings.HasSuffix(strings.ToLower(e.Name()), ".exe") {
			binaries = append(binaries, filepath.Join(dir, e.Name()))
		}
	}
	return binaries, nil
}

// normalizeVersion removes the 'v' prefix from versions for consistent storage
func normalizeVersion(version string) string {
	if strings.HasPrefix(version, "v") {
		return version[1:]
	}
	return version
}
