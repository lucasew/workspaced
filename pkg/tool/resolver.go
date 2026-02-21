package tool

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"sort"

	"workspaced/pkg/semver"
)

// EnsureInstalled ensures the tool is installed and returns the path to the executable binary.
func (m *Manager) EnsureInstalled(ctx context.Context, toolSpecStr, cmdName string) (string, error) {
	spec, err := ParseToolSpec(toolSpecStr)
	if err != nil {
		return "", err
	}

	// Handle "latest" version resolution
	actualVersion := spec.Version
	if spec.Version == "latest" {
		// Try to find any installed version locally first
		installed, err := m.FindInstalledVersions(spec)
		if err == nil && len(installed) > 0 {
			actualVersion = installed[0]
			spec.Version = actualVersion
		} else {
			// No local version found, resolve from provider
			resolved, err := m.ResolveLatestVersion(ctx, spec)
			if err != nil {
				return "", fmt.Errorf("failed to resolve latest version: %w", err)
			}
			actualVersion = resolved
			spec.Version = actualVersion
		}
	}

	// Try to resolve the binary first (if already installed)
	binPath, err := m.ResolveBinary(spec, cmdName)
	if err == nil {
		return binPath, nil
	}

	// Not found, check if we need to install
	normalizedVersion := normalizeVersion(actualVersion)
	versionDir := filepath.Join(m.toolsDir, spec.Dir(), normalizedVersion)

	if _, statErr := os.Stat(versionDir); os.IsNotExist(statErr) {
		slog.Info("tool not installed, installing", "spec", spec)

		if err := m.installWithHint(ctx, spec.String(), cmdName); err != nil {
			return "", fmt.Errorf("failed to install tool: %w", err)
		}

		// Try to resolve again
		binPath, err = m.ResolveBinary(spec, cmdName)
		if err != nil {
			return "", fmt.Errorf("tool installed but binary not found: %w", err)
		}
		return binPath, nil
	}

	// The version directory exists but the expected binary is missing.
	// Reinstalling with a binary hint fixes ambiguous artifact selections.
	slog.Info("tool version present but binary missing, reinstalling with hint", "spec", spec, "cmd", cmdName)
	if err := m.installWithHint(ctx, spec.String(), cmdName); err != nil {
		return "", fmt.Errorf("failed to reinstall tool: %w", err)
	}
	binPath, err = m.ResolveBinary(spec, cmdName)
	if err != nil {
		return "", fmt.Errorf("tool reinstalled but binary not found: %w", err)
	}
	return binPath, nil
}

// ResolveBinary attempts to find the executable binary for a specific tool version.
func (m *Manager) ResolveBinary(spec ToolSpec, cmdName string) (string, error) {
	normalizedVersion := normalizeVersion(spec.Version)
	versionDir := filepath.Join(m.toolsDir, spec.Dir(), normalizedVersion)

	if _, err := os.Stat(versionDir); os.IsNotExist(err) {
		return "", fmt.Errorf("tool directory not found: %s", versionDir)
	}

	candidates := []string{
		filepath.Join(versionDir, "bin", cmdName),
		filepath.Join(versionDir, "bin", cmdName+".exe"),
		filepath.Join(versionDir, cmdName),
		filepath.Join(versionDir, cmdName+".exe"),
	}

	for _, path := range candidates {
		if _, err := os.Stat(path); err == nil {
			return path, nil
		}
	}

	return "", fmt.Errorf("binary %q not found in %s", cmdName, versionDir)
}

// FindInstalledVersions returns a sorted list of installed versions for a tool.
func (m *Manager) FindInstalledVersions(spec ToolSpec) ([]string, error) {
	pkgDir := filepath.Join(m.toolsDir, spec.Dir())
	entries, err := os.ReadDir(pkgDir)
	if err != nil {
		return nil, err
	}

	var versions semver.SemVers
	for _, entry := range entries {
		if entry.IsDir() {
			versions = append(versions, semver.Parse(entry.Name()))
		}
	}

	if len(versions) == 0 {
		return nil, fmt.Errorf("no installed versions found")
	}

	sort.Sort(sort.Reverse(versions))

	var result []string
	for _, v := range versions {
		result = append(result, v.String())
	}
	return result, nil
}

// ResolveLatestVersion queries the provider to find the latest version of a package.
func (m *Manager) ResolveLatestVersion(ctx context.Context, spec ToolSpec) (string, error) {
	provider, err := GetProvider(spec.Provider)
	if err != nil {
		return "", err
	}

	pkgConfig, err := provider.ParsePackage(spec.Package)
	if err != nil {
		return "", err
	}

	versions, err := provider.ListVersions(ctx, pkgConfig)
	if err != nil {
		return "", err
	}

	if len(versions) == 0 {
		return "", fmt.Errorf("no versions found")
	}

	// Provider returns versions, we assume the first one is relevant (often latest)
	// TODO: Add proper sorting/semver logic if provider doesn't guarantee order
	return versions[0], nil
}

// EnsureAndRun simplifies running a tool by ensuring it's installed and returning an exec.Cmd.
func EnsureAndRun(ctx context.Context, toolSpecStr, cmdName string, args ...string) (*exec.Cmd, error) {
	m, err := NewManager()
	if err != nil {
		return nil, fmt.Errorf("failed to create tool manager: %w", err)
	}

	binPath, err := m.EnsureInstalled(ctx, toolSpecStr, cmdName)
	if err != nil {
		return nil, fmt.Errorf("failed to ensure tool installed: %w", err)
	}

	return exec.CommandContext(ctx, binPath, args...), nil
}
