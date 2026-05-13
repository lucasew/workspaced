package registry

import (
	"context"
	"fmt"
	"workspaced/pkg/tool"
	"workspaced/pkg/tool/provider"
)

func init() {
	tool.RegisterProvider(&Provider{})
}

// Provider is a placeholder for future registry implementation
// Currently returns "not implemented" for all operations
type Provider struct{}

func (p *Provider) ID() string   { return "registry" }
func (p *Provider) Name() string { return "Tool Registry (not implemented)" }

func (p *Provider) ParsePackage(spec string) (provider.PackageConfig, error) {
	return provider.PackageConfig{}, fmt.Errorf("registry provider not implemented yet - use explicit provider like 'github:%s'", spec)
}

func (p *Provider) ListVersions(ctx context.Context, pkg provider.PackageConfig) ([]string, error) {
	return nil, fmt.Errorf("registry provider not implemented")
}

func (p *Provider) GetArtifacts(ctx context.Context, pkg provider.PackageConfig, version string) ([]provider.Artifact, error) {
	return nil, fmt.Errorf("registry provider not implemented")
}

func (p *Provider) Install(ctx context.Context, artifact provider.Artifact, destPath string) error {
	return fmt.Errorf("registry provider not implemented")
}
