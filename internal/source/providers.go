package source

import (
	"context"

	"github.com/lucasew/workspaced/internal/configcue"
	"github.com/lucasew/workspaced/pkg/filespine"
)

// CueFiles contributes workspaced.file decls.
type CueFiles struct {
	Config *configcue.Config
}

func (p CueFiles) Name() string { return "cue:file" }

func (p CueFiles) Provide(ctx context.Context) (filespine.Patch, error) {
	_ = ctx
	m, err := p.Config.FileMap()
	if err != nil {
		return filespine.Patch{}, err
	}
	return filespine.Patch{Files: m}, nil
}

// FileSlots lowers already-discovered files into dest slots.
type FileSlots struct {
	Label string
	Files []File
}

func (p FileSlots) Name() string {
	if p.Label != "" {
		return p.Label
	}
	return "files"
}

func (p FileSlots) Provide(ctx context.Context) (filespine.Patch, error) {
	_ = ctx
	slots := make([]filespine.Contribution, 0, len(p.Files))
	for _, f := range p.Files {
		c, err := lowerFile(f)
		if err != nil {
			return filespine.Patch{}, err
		}
		slots = append(slots, c)
	}
	return filespine.Patch{Slots: slots}, nil
}

func splitTemplateFiles(files []File) (static, tmpl []File) {
	for _, f := range files {
		if filespine.IsTemplatePath(f.RelPath()) {
			tmpl = append(tmpl, f)
			continue
		}
		static = append(static, f)
	}
	return static, tmpl
}
