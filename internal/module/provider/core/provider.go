package core

import (
	"context"
	"fmt"

	"github.com/lucasew/workspaced/internal/module"
)

func init() {
	module.RegisterProvider(&Provider{})
}

// Provider is the "core" module provider. It dispatches to registered
// CoreModules based on the ref (e.g. "place" for core:place).
type Provider struct{}

func (p *Provider) ID() string   { return "core" }
func (p *Provider) Name() string { return "Core Module Provider" }

func (p *Provider) Resolve(ctx context.Context, req module.ResolveRequest) ([]module.ResolvedFile, error) {
	cm, ok := module.GetCoreModule(req.Ref)
	if !ok {
		return nil, fmt.Errorf("unknown core module %q", req.Ref)
	}
	return cm.Resolve(ctx, req)
}
