package registry

import (
	"context"
	"fmt"
	"strings"
	"workspaced/pkg/tool"
	"workspaced/pkg/tool/provider"
)

func init() {
	tool.Register("registry", &Provider{})
}

// Provider is a placeholder for a future central tool registry.
// It implements the thin handler interface and can later compose Tools
// from other backends (github, mise, pypi, etc.) using their exposed constructors.
type Provider struct{}

func (p *Provider) Name() string { return "Tool Registry (not implemented)" }

func (p *Provider) Tool(ref string) (provider.Tool, error) {
	return NewTool(ref)
}

// ParsePackage etc. kept only for any transitional direct use.
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

// ============================================================================
// RegistryTool - placeholder
// ============================================================================

// RegistryTool is a placeholder Tool for the future central registry.
// Exported with NewTool so other code (or the registry itself during development)
// can construct it.
type RegistryTool struct {
	ref string
}

// NewTool returns a non-functional Tool for the registry provider.
func NewTool(ref string) (provider.Tool, error) {
	return &RegistryTool{ref: strings.TrimSpace(ref)}, nil
}

func (t *RegistryTool) ListVersions(ctx context.Context) ([]string, error) {
	return nil, fmt.Errorf("registry tool %q not implemented", t.ref)
}

func (t *RegistryTool) Install(ctx context.Context, version string, destDir string) error {
	return fmt.Errorf("registry tool %q@%s not implemented (destDir=%s)", t.ref, version, destDir)
}
