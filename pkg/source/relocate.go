package source

import "context"

// RelocatePlugin forces all files discovered from a dotfiles repo
// (config tree + modules) to use a single physical target root.
//
// This is useful when you want to reinterpret the "output layout"
// (the RelPaths) under a different base than what the modules/providers
// originally emitted (e.g. apply the same declarations that normally
// target $HOME, but write them under the repo root instead).
type RelocatePlugin struct {
	target string
}

// NewRelocatePlugin returns a plugin that makes TargetBase() return target
// for every file, while keeping their RelPath and content.
func NewRelocatePlugin(target string) *RelocatePlugin {
	return &RelocatePlugin{target: target}
}

func (p *RelocatePlugin) Name() string {
	return "relocate"
}

func (p *RelocatePlugin) Process(ctx context.Context, files []File) ([]File, error) {
	out := make([]File, len(files))
	for i, f := range files {
		out[i] = &relocatedFile{File: f, target: p.target}
	}
	return out, nil
}

type relocatedFile struct {
	File
	target string
}

func (f *relocatedFile) TargetBase() string {
	return f.target
}
