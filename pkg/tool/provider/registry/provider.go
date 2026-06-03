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
// from other backends (github, pypi, etc.) using their exposed constructors.
// Currently it only provides a curated list of github-backed named tools.
type Provider struct{}

func (p *Provider) Name() string { return "Tool Registry (not implemented)" }

func (p *Provider) Tool(ref string) (provider.Tool, error) {
	// Inline dispatch for named tools (the "registry" behavior).
	// See namedTools in applications.go for the curated map of github tools.
	name := strings.TrimSpace(ref)
	if name == "" {
		return nil, fmt.Errorf("registry tool name cannot be empty")
	}

	if ctor, ok := namedTools[name]; ok {
		return ctor()
	}

	return nil, fmt.Errorf("unknown named tool %q (registry only knows curated github tools; use explicit 'mise:' or 'github:' provider instead)", name)
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

// NewTool constructs a Tool for a named entry in the registry.
// It delegates to the Provider so the dispatch logic is not duplicated.
func NewTool(ref string) (provider.Tool, error) {
	// For direct construction of "registry:foo", we go through the same
	// named dispatch. This makes `registry.NewTool("uv")` do the right thing.
	return (&Provider{}).Tool(ref)
}

// RegistryTool is a thin wrapper type if someone wants to type-assert the
// registry origin. In practice we usually return the inner backend Tool
// directly (so ArtifactTool / BinaryTool assertions work without extra wrappers).
// We keep a type for documentation / future use.
type RegistryTool struct {
	inner provider.Tool
	name  string
}

func (t *RegistryTool) ListVersions(ctx context.Context) ([]string, error) {
	return t.inner.ListVersions(ctx)
}

func (t *RegistryTool) Install(ctx context.Context, version string, destDir string) error {
	return t.inner.Install(ctx, version, destDir)
}

// Note: if the inner tool implements ArtifactTool or BinaryTool, the
// assertions will succeed on the inner value, not on RegistryTool.
// If you need the registry wrapper to forward the extra interfaces,
// you can embed or use a more sophisticated wrapper.

