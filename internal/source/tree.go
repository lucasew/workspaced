package source

import (
	"context"

	"github.com/lucasew/workspaced/internal/configcue"
	"github.com/lucasew/workspaced/internal/filespine"
	"github.com/lucasew/workspaced/internal/template"
	"github.com/lucasew/workspaced/pkg/logging"
)

// Tree is dest files after templates and config unify.
// Apply writes this; it does not rebuild it.
type Tree struct {
	dest       *filespine.FS
	targetBase string
	files      []File
	Warnings   []string
}

// Dest is the encoded dest tree. Open returns the combined file.
func (t *Tree) Dest() *filespine.FS {
	if t == nil {
		return nil
	}
	return t.dest
}

// Files is the apply view of Dest (paths relative to TargetBase).
func (t *Tree) Files() []File {
	if t == nil {
		return nil
	}
	return t.files
}

// TargetBase is the default physical root for dest paths.
func (t *Tree) TargetBase() string {
	if t == nil {
		return ""
	}
	return t.targetBase
}

// TreeFromFiles builds a Tree that only has an apply view (no dest FS).
// Tests and callers that already have desired files use this.
func TreeFromFiles(files []File, warnings ...string) *Tree {
	return &Tree{files: files, Warnings: warnings}
}

// BuildTree renders providers through templates and workspaced.file.
// Providers only discover or adjust files (scan, modules, relocate).
func BuildTree(ctx context.Context, cfg *configcue.Config, targetBase string, providers ...Plugin) (*Tree, error) {
	var warnings []string
	ctx = WithWarningSink(ctx, &warnings)

	p := NewPipeline(providers...)
	p.AddPlugin(NewTemplateExpanderPlugin(template.NewEngine(ctx), cfg))
	files, err := p.Run(ctx, nil)
	if err != nil {
		return nil, err
	}
	tree, err := buildTreeFromFiles(ctx, treeFromFiles{
		cfg:        cfg,
		targetBase: targetBase,
		files:      files,
	})
	if err != nil {
		return nil, err
	}
	tree.Warnings = warnings
	return tree, nil
}

// BuildStandardTree is BuildTree with the usual config-tree + modules sources.
func BuildStandardTree(ctx context.Context, cfg *configcue.Config, opts StandardDotfilesOptions) (*Tree, error) {
	providers, err := standardProviders(opts)
	if err != nil {
		return nil, err
	}
	return BuildTree(ctx, cfg, standardTarget(opts), providers...)
}

type treeFromFiles struct {
	cfg        *configcue.Config
	targetBase string
	files      []File
}

func buildTreeFromFiles(ctx context.Context, in treeFromFiles) (*Tree, error) {
	logger := logging.GetLogger(ctx)
	declared := map[string]filespine.File{}
	if in.cfg != nil {
		var err error
		declared, err = filespine.Parse(filespine.LookupFile(in.cfg.Cue()))
		if err != nil {
			return nil, err
		}
	}
	extras := make([]filespine.Contribution, 0, len(in.files))
	for _, f := range in.files {
		c, err := lowerFile(f)
		if err != nil {
			return nil, err
		}
		extras = append(extras, c)
	}
	merged, err := filespine.Merge(declared, extras)
	if err != nil {
		return nil, err
	}
	dest := filespine.NewFS(merged)
	out := make([]File, 0, len(merged))
	for _, decl := range dest.Files() {
		base := decl.TargetBase
		if base == "" {
			base = in.targetBase
		}
		sf, err := destFile(decl, base)
		if err != nil {
			return nil, err
		}
		out = append(out, sf)
	}
	logger.Debug("file spine encoded", "files", len(out))
	return &Tree{dest: dest, targetBase: in.targetBase, files: out}, nil
}
