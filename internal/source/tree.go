package source

import (
	"context"

	"github.com/lucasew/workspaced/internal/configcue"
	"github.com/lucasew/workspaced/internal/template"
	"github.com/lucasew/workspaced/pkg/filespine"
	"github.com/lucasew/workspaced/pkg/logging"
)

// Tree is dest files after providers compose.
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

// Builder discovers source files, then composes cue + static + templates.
type Builder struct {
	Config     *configcue.Config
	TargetBase string
	Providers  []Plugin
}

// Tree runs discovery, then filespine.Compose of cue, static, and templates.
func (b Builder) Tree(ctx context.Context) (*Tree, error) {
	var warnings []string
	ctx = WithWarningSink(ctx, &warnings)

	files, err := NewPipeline(b.Providers...).Run(ctx, nil)
	if err != nil {
		return nil, err
	}
	static, tmpl := splitTemplateFiles(files)
	rendered, err := NewTemplateExpanderPlugin(template.NewEngine(ctx), b.Config).Process(ctx, tmpl)
	if err != nil {
		return nil, err
	}
	dest, err := filespine.Compose(ctx,
		CueFiles{Config: b.Config},
		FileSlots{Label: "static", Files: static},
		FileSlots{Label: "templates", Files: rendered},
	)
	if err != nil {
		return nil, err
	}
	tree, err := NewTree(dest, b.TargetBase)
	if err != nil {
		return nil, err
	}
	tree.Warnings = warnings
	logging.GetLogger(ctx).Debug("dest composed", "files", len(tree.files))
	return tree, nil
}
