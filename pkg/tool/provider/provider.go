package provider

import (
	"context"
)

type Provider interface {
	ID() string
	Name() string
	ParsePackage(spec string) (PackageConfig, error)
	ListVersions(ctx context.Context, pkg PackageConfig) ([]string, error)
	GetArtifacts(ctx context.Context, pkg PackageConfig, version string) ([]Artifact, error)
	Install(ctx context.Context, artifact Artifact, destPath string) error
}

type PackageConfig struct {
	Provider string
	Spec     string
	Repo     string
}

type Artifact struct {
	OS   string
	Arch string
	URL  string
	Hash string
}
