package source

import (
	"context"
	"fmt"
	"workspaced/pkg/logging"
)

// Plugin processes a list of files and returns a new list.
type Plugin interface {
	// Name returns the plugin name (for logging).
	Name() string

	// Process transforms a list of files.
	// It can add, remove, or modify files.
	Process(ctx context.Context, files []File) ([]File, error)
}

// Pipeline executes a sequence of plugins.
type Pipeline struct {
	plugins []Plugin
}

// NewPipeline creates a pipeline with the given plugins.
func NewPipeline(plugins ...Plugin) *Pipeline {
	return &Pipeline{plugins: plugins}
}

// AddPlugin appends a plugin to the end of the pipeline.
func (p *Pipeline) AddPlugin(plugin Plugin) {
	p.plugins = append(p.plugins, plugin)
}

// Run executes the full pipeline.
func (p *Pipeline) Run(ctx context.Context, initial []File) ([]File, error) {
	logger := logging.GetLogger(ctx)
	current := initial

	for i, plugin := range p.plugins {
		logger.Debug("running plugin", "index", i, "name", plugin.Name(), "input_count", len(current))

		result, err := plugin.Process(ctx, current)
		if err != nil {
			return nil, fmt.Errorf("plugin %s failed: %w", plugin.Name(), err)
		}

		logger.Debug("plugin completed", "name", plugin.Name(), "output_count", len(result))
		current = result
	}

	logger.Info("pipeline completed", "total_plugins", len(p.plugins), "final_count", len(current))
	return current, nil
}

// GetPlugins returns the list of configured plugins.
func (p *Pipeline) GetPlugins() []Plugin {
	return p.plugins
}
