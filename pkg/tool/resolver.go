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

// EnsureInstalled resolves the binary path for a requested tool, triggering a dynamic
// installation if the tool or its explicit version is missing locally. It checks "latest"
// tags against upstream providers and gracefully falls back to hinted installations
// if artifact boundaries are ambiguous.
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

// ResolveBinary attempts to locate the executable binary for an already-installed tool.
// It searches standard bin paths within the localized version directory, accounting for
// platform-specific extensions like ".exe" on Windows.
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

// ResolveLatestVersion delegates to the underlying tool provider to query the remote
// registry for the most recent valid version. It assumes the first returned version
// is the latest, matching standard provider behaviors.
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

// EnsureAndRun serves as a top-level helper to both resolve (and potentially install)
// a tool, and immediately bind it to an *exec.Cmd configured with the given arguments.
// This is the common entry point for direct, non-lazy tool invocations.
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
