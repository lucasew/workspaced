package source

import (
	"context"
	"path/filepath"
)

// KeepTargetPlugin keeps files whose TargetBase matches target.
type KeepTargetPlugin struct {
	target string
}

// NewKeepTargetPlugin returns a plugin that drops files not rooted at target.
func NewKeepTargetPlugin(target string) *KeepTargetPlugin {
	return &KeepTargetPlugin{target: filepath.Clean(target)}
}

func (p *KeepTargetPlugin) Name() string { return "keep-target" }

func (p *KeepTargetPlugin) Process(ctx context.Context, files []File) ([]File, error) {
	return filterTarget(files, func(base string) bool {
		return base == p.target
	}), nil
}

// DropTargetPlugin drops files whose TargetBase matches target.
type DropTargetPlugin struct {
	target string
}

// NewDropTargetPlugin returns a plugin that drops files rooted at target.
func NewDropTargetPlugin(target string) *DropTargetPlugin {
	return &DropTargetPlugin{target: filepath.Clean(target)}
}

func (p *DropTargetPlugin) Name() string { return "drop-target" }

func (p *DropTargetPlugin) Process(ctx context.Context, files []File) ([]File, error) {
	return filterTarget(files, func(base string) bool {
		return base != p.target
	}), nil
}

func filterTarget(files []File, keep func(base string) bool) []File {
	out := make([]File, 0, len(files))
	for _, f := range files {
		if keep(filepath.Clean(f.TargetBase())) {
			out = append(out, f)
		}
	}
	return out
}
