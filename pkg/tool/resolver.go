package tool

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	execdriver "workspaced/pkg/driver/exec"
	"workspaced/pkg/logging"
	parsespec "workspaced/pkg/parse/spec"
	"workspaced/pkg/taskgroup"
	"workspaced/pkg/tool/backend"
)

var (
	// ErrNoVersionsFound is returned when a tool has no available versions upstream.
	ErrNoVersionsFound = errors.New("no versions found")
	// ErrToolDirNotFound is returned when a tool's version directory doesn't exist.
	ErrToolDirNotFound = errors.New("tool directory not found")
	// ErrBinaryNotFound is returned when the binary is not found within a tool's install dir.
	ErrBinaryNotFound = errors.New("binary not found")
)

// EnsureInstalled resolves the binary path for a requested tool, triggering a dynamic
// installation if the version folder is missing or empty. "latest" is resolved against
// the backend first when needed. The install decision is simply: if the version
// directory exists and is not empty, treat the tool as installed (no refetch).
// FindBinary is only used to locate the specific cmdName inside an existing folder.
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

	normalizedVersion := normalizeVersion(actualVersion)
	versionDir := filepath.Join(m.toolsDir, spec.Dir(), normalizedVersion)

	logger := logging.GetLogger(ctx)

	// Just check if the folder is there and it's not empty.
	// If a version directory exists and has any contents, treat the tool
	// version as installed (prevents the fetcher from refetching over and over
	// due to exact binary name mismatches, prior non-hinted installs, etc.).
	// We still use FindBinary only to *locate* the requested cmd inside it.
	if entries, err := os.ReadDir(versionDir); err == nil && len(entries) > 0 {
		if binPath := FindBinary(versionDir, cmdName); binPath != "" {
			return binPath, nil
		}
		return "", fmt.Errorf("%w: %q in %s", ErrBinaryNotFound, cmdName, versionDir)
	}

	// Folder missing or empty: perform the install.
	logger.Info("installing tool", "spec", spec, "provider", spec.Provider, "version", actualVersion, "bin", cmdName)

	if bt, ok := t.(backend.BinaryTool); ok {
		var binPath string
		var installErr error
		if parent := taskgroup.FromContext(ctx); parent != nil {
			child, _ := parent.SubGroup(ctx)
			child.Go("install:"+spec.String(), taskgroup.Internet, func(ctx context.Context, s *taskgroup.Status) error {
				s.Update("installing " + normalizedVersion)
				s.Progress(0, 1)
				binPath, installErr = bt.EnsureBinary(ctx, actualVersion, cmdName, versionDir)
				s.Progress(1, 1)
				return installErr
			})
			if werr := child.Wait(); werr != nil && installErr == nil {
				installErr = werr
			}
		} else {
			binPath, installErr = bt.EnsureBinary(ctx, actualVersion, cmdName, versionDir)
		}
		if installErr != nil {
			return "", fmt.Errorf("failed to install tool: %w", installErr)
		}
		logger.Info("tool installed", "spec", spec, "bin_path", binPath)
		return binPath, nil
	}

	if err := m.installWithHint(ctx, spec.String(), cmdName); err != nil {
		return "", fmt.Errorf("failed to install tool: %w", err)
	}

	// Try to resolve again after a fresh install.
	binPath, err := m.ResolveBinary(spec, cmdName)
	if err != nil {
		return "", fmt.Errorf("tool installed but binary not found: %w", err)
	}
	logger.Info("tool installed", "spec", spec, "bin_path", binPath)
	return binPath, nil
}

// ResolveBinary attempts to locate the executable binary for an already-installed tool.
// It searches standard bin paths within the localized version directory, accounting for
// platform-specific extensions like ".exe" on Windows.
func (m *Manager) ResolveBinary(spec parsespec.Spec, cmdName string) (string, error) {
	normalizedVersion := normalizeVersion(spec.Version)
	versionDir := filepath.Join(m.toolsDir, spec.Dir(), normalizedVersion)

	if _, err := os.Stat(versionDir); os.IsNotExist(err) {
		return "", fmt.Errorf("%w: %s", ErrToolDirNotFound, versionDir)
	}

	if binPath := FindBinary(versionDir, cmdName); binPath != "" {
		return binPath, nil
	}

	return "", fmt.Errorf("%w: %q in %s", ErrBinaryNotFound, cmdName, versionDir)
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
		return "", ErrNoVersionsFound
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
