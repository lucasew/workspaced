package registry

import (
	"context"
	"fmt"
	"strings"

	"workspaced/pkg/modfile"
	"workspaced/pkg/tool"
	"workspaced/pkg/tool/provider"
)

func init() {
	tool.Register("registry", &Provider{})
}

var namedTools = map[string]func() (provider.Tool, error){}

func RegisterRegistryTool(name string, f func() (provider.Tool, error)) {
	if _, ok := namedTools[name]; ok {
		panic(fmt.Sprintf("registry: tool %s is being defined twice", name))
	}
	namedTools[name] = f
}

// Provider is a placeholder for a future central tool registry.
// It implements the thin handler interface and can later compose Tools
// from other backends (github, pypi, etc.) using their exposed constructors.
// Currently it only provides a curated list of github-backed named tools (by short name). Bare tool specs without a provider: default to registry.
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

	return nil, fmt.Errorf("unknown named tool %q (registry only knows curated short names for github tools; bare names default to the registry provider; use explicit 'mise:xxx' or 'github:owner/repo' for other tools)", name)
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
// We keep a type for documentation / future use. It forwards EnrichLockfile
// to the inner.
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

// EnrichLockfile forwards to the inner tool. The concrete implementation
// receives the actual *modfile.RenovateDependency and can mutate it.
func (t *RegistryTool) EnrichLockfile(entry *modfile.RenovateDependency) {
	t.inner.EnrichLockfile(entry)
}

// Note: if the inner tool implements ArtifactTool or BinaryTool, the
// assertions will succeed on the inner value, not on RegistryTool.
// If you need the registry wrapper to forward the extra interfaces,
// you can embed or use a more sophisticated wrapper.
