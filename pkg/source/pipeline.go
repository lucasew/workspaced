package source

import (
	"context"
	"fmt"

	"workspaced/pkg/configcue"
	"workspaced/pkg/logging"
	"workspaced/pkg/template"
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

// StandardDotfilesOptions controls how to build a pipeline that
// processes a typical dotfiles repo (direct config tree + modules).
type StandardDotfilesOptions struct {
	// ConfigTreeDir is the directory containing the direct file tree
	// (usually "<dotfiles>/config"). Files here are placed according to
	// their relative path (with .tmpl / .d.tmpl processing).
	// If empty, this source is not used.
	ConfigTreeDir string

	// ConfigTreeTarget is the TargetBase for files coming from ConfigTreeDir.
	ConfigTreeTarget string

	// ModulesDir, if non-empty, will cause a ModuleScannerPlugin to be added
	// (with priority 100).
	ModulesDir string
	// ModulesCfg is the config passed to the module scanner.
	ModulesCfg *configcue.Config

	// RelocateTo, if non-empty, adds a RelocatePlugin early (right after
	// scanners). This forces *all* files (config tree + modules) to use this
	// physical root, interpreting their RelPaths relative to it.
	RelocateTo string
}

// NewStandardDotfilesPipeline builds the common plugin sequence used by
// "home apply", "codebase apply", and similar flows:
//
//   - (optional) direct config tree (the "config/" directory with .tmpl rules)
//   - (optional) module scanner
//   - (optional) relocate plugin
//   - template expander
//   - dotd processor
//   - strict conflict resolver
//
// It does *not* add home-specific things like the dconf provider.
//
// The "config tree" is just one way to provide files (with the familiar
// template conventions). Modules are another. Both end up as File entries
// that get placed according to their final TargetBase + RelPath.
func NewStandardDotfilesPipeline(
	ctx context.Context,
	cfg *configcue.Config,
	opts StandardDotfilesOptions,
) (*Pipeline, error) {

	p := NewPipeline()

	if opts.ConfigTreeDir != "" {
		scanner, err := NewScannerPlugin(ScannerConfig{
			Name:       "config-tree",
			BaseDir:    opts.ConfigTreeDir,
			TargetBase: opts.ConfigTreeTarget,
			Priority:   50,
		})
		if err != nil {
			return nil, err
		}
		p.AddPlugin(scanner)
	}

	// Note: we add the module scanner even if opts.ModulesDir does not
	// exist on disk. This is required for pure core:place (and similar)
	// modules that don't require a local modules/ checkout.
	if opts.ModulesDir != "" && opts.ModulesCfg != nil {
		p.AddPlugin(NewModuleScannerPlugin(opts.ModulesDir, opts.ModulesCfg, 100))
	}

	if opts.RelocateTo != "" {
		p.AddPlugin(NewRelocatePlugin(opts.RelocateTo))
	}

	engine := template.NewEngine(ctx)

	p.AddPlugin(NewTemplateExpanderPlugin(engine, cfg))
	p.AddPlugin(NewDotDProcessorPlugin(engine, cfg))
	p.AddPlugin(NewStrictConflictResolverPlugin())

	return p, nil
}
