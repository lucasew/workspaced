package apps

import (
	"context"
	"fmt"
	"runtime"
	"strings"

	"github.com/lucasew/workspaced/internal/tool/backend"
	providerinstall "github.com/lucasew/workspaced/internal/tool/backend/install"
	"github.com/lucasew/workspaced/internal/tool/checks"
)

// normalizeVersion trims space and the given prefixes (each once, in order).
// Empty and "latest" are returned unchanged after that strip.
func normalizeVersion(version string, prefixes ...string) string {
	v := strings.TrimSpace(version)
	for _, p := range prefixes {
		v = strings.TrimPrefix(v, p)
	}
	if v == "" || v == "latest" {
		return v
	}
	return v
}

// normalizeVPrefixed strips a leading v/V (Flutter, CMake, …).
func normalizeVPrefixed(version string) string {
	return normalizeVersion(version, "v", "V")
}

// normalizeV strips a leading v only (esbuild, terraform, …).
func normalizeV(version string) string {
	return normalizeVersion(version, "v")
}

// defaultInstallArtifact is the usual InstallArtifact body for registry tools
// that download via providerinstall with default DownloadOptions.
func defaultInstallArtifact(ctx context.Context, artifact backend.Artifact, destDir string) error {
	return providerinstall.InstallArtifact(ctx, artifact, destDir, providerinstall.DownloadOptions{})
}

// ensureToolBinary is the usual EnsureBinary body: install then locate cmdName.
func ensureToolBinary(ctx context.Context, version, cmdName, destDir, label string, install func(context.Context, string, string) error) (string, error) {
	return checks.EnsureBinary(destDir, cmdName, label, func() error {
		return install(ctx, version, destDir)
	})
}

func resolveToolVersion(ctx context.Context, version string, normalize func(string) string, listVersions func(context.Context) ([]string, error)) (string, error) {
	v := normalize(version)
	if v != "" && v != "latest" {
		return v, nil
	}
	vers, err := listVersions(ctx)
	if err != nil {
		return "", err
	}
	if len(vers) == 0 {
		return "", ErrNoVersions
	}
	return normalize(vers[0]), nil
}

func installFirstArtifact(
	ctx context.Context,
	version, destDir string,
	normalize func(string) string,
	listVersions func(context.Context) ([]string, error),
	listArtifacts func(context.Context, string) ([]backend.Artifact, error),
	installArtifact func(context.Context, backend.Artifact, string) error,
) error {
	v, err := resolveToolVersion(ctx, version, normalize, listVersions)
	if err != nil {
		return err
	}
	arts, err := listArtifacts(ctx, v)
	if err != nil {
		return err
	}
	if len(arts) == 0 {
		return ErrNoPlatformArtifact
	}
	return installArtifact(ctx, arts[0], destDir)
}

func installSelectedArtifact(
	ctx context.Context,
	version, destDir, binaryHint, toolRef string,
	normalize func(string) string,
	listVersions func(context.Context) ([]string, error),
	listArtifacts func(context.Context, string) ([]backend.Artifact, error),
	installArtifact func(context.Context, backend.Artifact, string) error,
) error {
	v, err := resolveToolVersion(ctx, version, normalize, listVersions)
	if err != nil {
		return err
	}
	arts, err := listArtifacts(ctx, v)
	if err != nil {
		return err
	}
	if len(arts) == 0 {
		return ErrNoPlatformArtifact
	}
	artifact := backend.SelectArtifact(arts, runtime.GOOS, runtime.GOARCH, binaryHint)
	if artifact == nil {
		return fmt.Errorf("no suitable artifact found for %s/%s for %s@%s", runtime.GOOS, runtime.GOARCH, toolRef, v)
	}
	return installArtifact(ctx, *artifact, destDir)
}
