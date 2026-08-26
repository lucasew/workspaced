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

// NewTree wraps a dest FS and builds the apply view.
func NewTree(dest *filespine.FS, targetBase string) (*Tree, error) {
	if dest == nil {
		return &Tree{targetBase: targetBase}, nil
	}
	decls := dest.Files()
	out := make([]File, 0, len(decls))
	for _, decl := range decls {
		base := decl.TargetBase
		if base == "" {
			base = targetBase
		}
		sf, err := destFile(decl, base)
		if err != nil {
			return nil, err
		}
		out = append(out, sf)
	}
	return &Tree{dest: dest, targetBase: targetBase, files: out}, nil
}

// NewApplyTree is a dest-less Tree for callers that already have apply files.
func NewApplyTree(files []File, warnings ...string) *Tree {
	return &Tree{files: files, Warnings: warnings}
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

// Builder renders providers through templates and workspaced.file.
type Builder struct {
	Config     *configcue.Config
	TargetBase string
	Providers  []Plugin
}

// Tree runs providers, expands templates, and unifies workspaced.file.
func (b Builder) Tree(ctx context.Context) (*Tree, error) {
	var warnings []string
	ctx = WithWarningSink(ctx, &warnings)

	p := NewPipeline(b.Providers...)
	p.AddPlugin(NewTemplateExpanderPlugin(template.NewEngine(ctx), b.Config))
	files, err := p.Run(ctx, nil)
	if err != nil {
		return nil, err
	}
	tree, err := b.unify(ctx, files)
	if err != nil {
		return nil, err
	}
	tree.Warnings = warnings
	return tree, nil
}

func (b Builder) unify(ctx context.Context, files []File) (*Tree, error) {
	logger := logging.GetLogger(ctx)
	declared, err := b.Config.FileMap()
	if err != nil {
		return nil, err
	}
	extras := make([]filespine.Contribution, 0, len(files))
	for _, f := range files {
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
	tree, err := NewTree(filespine.NewFS(merged), b.TargetBase)
	if err != nil {
		return nil, err
	}
	logger.Debug("file spine encoded", "files", len(tree.files))
	return tree, nil
}
