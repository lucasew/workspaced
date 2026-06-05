package tool

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"

	execdriver "workspaced/pkg/driver/exec"
	parsespec "workspaced/pkg/parse/spec"
	"workspaced/pkg/tool/backend"
)

// EnsureInstalled ensures the tool is installed and returns the path to the executable binary.
func (m *Manager) EnsureInstalled(ctx context.Context, toolSpecStr, cmdName string) (string, error) {
	spec, err := parsespec.Parse(toolSpecStr)
	if err != nil {
		return "", err
	}

	// Handle "latest" version resolution.
	// In the direct path (used by "workspaced tool with" etc.), "latest"
	// always queries upstream. We never fall back to locally installed
	// versions for "latest" here (installed versions are only used when
	// an explicit non-latest version is not specified in some contexts).
	actualVersion := spec.Version
	if spec.Version == "latest" {
		resolved, err := m.ResolveLatestVersion(ctx, spec)
		if err != nil {
			return "", fmt.Errorf("failed to resolve latest version: %w", err)
		}
		actualVersion = resolved
		spec.Version = actualVersion
	}
	p, err := Get(spec.Provider)
	if err != nil {
		return "", err
	}
	t, err := p.Tool(spec.Package)
	if err != nil {
		return "", err
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
		slog.Info("installing tool", "spec", spec, "provider", spec.Provider, "version", actualVersion, "bin", cmdName)

		if bt, ok := t.(backend.BinaryTool); ok {
			binPath, err := bt.EnsureBinary(ctx, actualVersion, cmdName, versionDir)
			if err != nil {
				return "", fmt.Errorf("failed to install tool: %w", err)
			}
			slog.Info("tool installed", "spec", spec, "bin_path", binPath)
			return binPath, nil
		}

		if err := m.installWithHint(ctx, spec.String(), cmdName); err != nil {
			return "", fmt.Errorf("failed to install tool: %w", err)
		}

		// Try to resolve again
		binPath, err = m.ResolveBinary(spec, cmdName)
		if err != nil {
			return "", fmt.Errorf("tool installed but binary not found: %w", err)
		}
		slog.Info("tool installed", "spec", spec, "bin_path", binPath)
		return binPath, nil
	}

	// The version directory exists but the expected binary is missing.
	// Reinstalling with a binary hint fixes ambiguous artifact selections.
	slog.Info("reinstalling tool with binary hint", "spec", spec, "provider", spec.Provider, "version", actualVersion, "bin", cmdName)
	if bt, ok := t.(backend.BinaryTool); ok {
		binPath, err := bt.EnsureBinary(ctx, actualVersion, cmdName, versionDir)
		if err != nil {
			return "", err
		}
		slog.Info("tool reinstalled", "spec", spec, "bin_path", binPath)
		return binPath, nil
	}
	if err := m.installWithHint(ctx, spec.String(), cmdName); err != nil {
		return "", fmt.Errorf("failed to reinstall tool: %w", err)
	}
	binPath, err = m.ResolveBinary(spec, cmdName)
	if err != nil {
		return "", fmt.Errorf("tool reinstalled but binary not found: %w", err)
	}
	slog.Info("tool reinstalled", "spec", spec, "bin_path", binPath)
	return binPath, nil
}

// ResolveBinary attempts to find the executable binary for a specific tool version.
func (m *Manager) ResolveBinary(spec parsespec.Spec, cmdName string) (string, error) {
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

// ResolveLatestVersion queries the provider to find the latest version of a package.
func (m *Manager) ResolveLatestVersion(ctx context.Context, spec parsespec.Spec) (string, error) {
	p, err := Get(spec.Provider)
	if err != nil {
		return "", err
	}

	t, err := p.Tool(spec.Package)
	if err != nil {
		return "", err
	}

	versions, err := t.ListVersions(ctx)
	if err != nil {
		return "", err
	}

	if len(versions) == 0 {
		return "", fmt.Errorf("no versions found")
	}

	// We assume the first one is relevant (often latest from the provider).
	// TODO: Add proper sorting/semver logic if provider doesn't guarantee order.
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

	return execdriver.Run(ctx, binPath, args...)
}
