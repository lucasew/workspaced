package source

import (
	"context"
	"fmt"
)

// ProviderPlugin adapts a Provider to the Plugin interface.
// Allows legacy providers to be used in the pipeline system.
type ProviderPlugin struct {
	provider Provider
	priority int
}

// NewProviderPlugin creates a plugin from a legacy provider.
func NewProviderPlugin(provider Provider, priority int) *ProviderPlugin {
	return &ProviderPlugin{
		provider: provider,
		priority: priority,
	}
}

func (p *ProviderPlugin) Name() string {
	return fmt.Sprintf("provider:%s", p.provider.Name())
}

func (p *ProviderPlugin) Process(ctx context.Context, files []File) ([]File, error) {
	desired, err := p.provider.GetDesiredState(ctx)
	if err != nil {
		return nil, fmt.Errorf("provider %s failed: %w", p.provider.Name(), err)
	}

	// Convert DesiredState to source.File
	newFiles := make([]File, len(desired))
	for i, d := range desired {
		// Legacy providers always return BufferFiles or StaticFiles built from legacy DesiredState.
		// In the new model, DesiredState already contains a File interface.
		newFiles[i] = d.File
	}

	// Append to existing files
	return append(files, newFiles...), nil
}
