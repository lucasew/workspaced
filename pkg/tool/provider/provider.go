package provider

import (
	"context"
)

// Provider defines the interface for tool providers (e.g., GitHub Releases, specialized registries).
// It handles package parsing, version listing, artifact resolution, and installation.
type Provider interface {
	// ID returns the unique identifier for the provider (e.g., "github").
	ID() string

	// Name returns a human-readable name for the provider.
	Name() string

	// ParsePackage parses a provider-specific package specification string into a PackageConfig.
	// For example, the GitHub provider might parse "owner/repo".
	ParsePackage(spec string) (PackageConfig, error)

	// ListVersions returns a list of available versions for the given package.
	// The order of versions is provider-specific but typically chronological or semantic.
	ListVersions(ctx context.Context, pkg PackageConfig) ([]string, error)

	// GetArtifacts returns a list of downloadable artifacts for a specific version of the package.
	// This includes platform-specific binaries or archives.
	GetArtifacts(ctx context.Context, pkg PackageConfig, version string) ([]Artifact, error)

	// Install downloads and installs the specified artifact to the destination path.
	// It handles extraction (if the artifact is an archive) and ensures the binary is executable.
	Install(ctx context.Context, artifact Artifact, destPath string) error
}

// PackageConfig holds the configuration for a package as understood by a specific provider.
type PackageConfig struct {
	// Provider is the ID of the provider managing this package.
	Provider string
	// Spec is the original package specification string.
	Spec string
	// Repo is the repository identifier (provider-specific, e.g., "owner/repo" for GitHub).
	Repo string
}

// Artifact represents a downloadable file for a specific tool version and platform.
type Artifact struct {
	// OS is the operating system the artifact is built for (e.g., "linux", "darwin").
	OS string
	// Arch is the architecture the artifact is built for (e.g., "amd64", "arm64").
	Arch string
	// URL is the download URL for the artifact.
	URL string
	// Hash is the expected checksum of the artifact (optional, e.g., "sha256:abcdef...").
	Hash string
}
