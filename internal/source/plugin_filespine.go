package source

import (
	"context"
	"fmt"
	"io"
	"path"
	"path/filepath"
	"strings"

	"github.com/lucasew/workspaced/internal/configcue"
	"github.com/lucasew/workspaced/internal/filespine"
)

const contentKey = "content"
const refKey = "src"

// FileSpinePlugin lowers pipeline files into workspaced.file, unifies with
// CUE declarations, and emits dest files. Open on the dest FS is combined.
type FileSpinePlugin struct {
	cfg        *configcue.Config
	targetBase string
}

func NewFileSpinePlugin(cfg *configcue.Config, targetBase string) *FileSpinePlugin {
	return &FileSpinePlugin{cfg: cfg, targetBase: targetBase}
}

func (p *FileSpinePlugin) Name() string { return "file-spine" }

func (p *FileSpinePlugin) Process(ctx context.Context, files []File) ([]File, error) {
	tree, err := buildTreeFromFiles(ctx, treeFromFiles{
		cfg:        p.cfg,
		targetBase: p.targetBase,
		files:      files,
	})
	if err != nil {
		return nil, err
	}
	return tree.Files(), nil
}

func lowerFile(f File) (filespine.Contribution, error) {
	rel := filepath.ToSlash(f.RelPath())
	c := filespine.Contribution{
		Mode:       f.Mode(),
		Info:       f.SourceInfo(),
		Module:     moduleNameOf(f),
		TargetBase: f.TargetBase(),
		Symlink:    f.Type() == TypeSymlink,
	}
	if i := strings.Index(rel, ".d.tmpl/"); i >= 0 {
		c.Path = strings.Trim(rel[:i], "/")
		c.Type = filespine.TypeLines
		c.Key = path.Base(rel)
		slot, err := slotFromFile(f)
		if err != nil {
			return filespine.Contribution{}, fmt.Errorf("lower %s: %w", f.SourceInfo(), err)
		}
		c.Slot = slot
		if c.Path == "" {
			return filespine.Contribution{}, fmt.Errorf("lower %s: %w", f.SourceInfo(), filespine.ErrEmptyPath)
		}
		return c, nil
	}
	c.Path = strings.Trim(rel, "/")
	if c.Path == "" {
		return filespine.Contribution{}, fmt.Errorf("lower %s: %w", f.SourceInfo(), filespine.ErrEmptyPath)
	}
	if sf, ok := f.(*StaticFile); ok && sf.AbsPath != "" {
		c.Type = filespine.TypeRef
		c.Key = refKey
		c.Slot = filespine.Slot{Kind: filespine.KindRef, Ref: sf.AbsPath}
		return c, nil
	}
	c.Type = filespine.TypeText
	c.Key = contentKey
	slot, err := slotFromFile(f)
	if err != nil {
		return filespine.Contribution{}, fmt.Errorf("lower %s: %w", f.SourceInfo(), err)
	}
	c.Slot = slot
	return c, nil
}

func slotFromFile(f File) (filespine.Slot, error) {
	if sf, ok := f.(*StaticFile); ok && sf.AbsPath != "" {
		return filespine.Slot{Kind: filespine.KindRef, Ref: sf.AbsPath}, nil
	}
	r, err := f.Reader()
	if err != nil {
		return filespine.Slot{}, err
	}
	data, err := io.ReadAll(r)
	closeErr := r.Close()
	if err != nil {
		return filespine.Slot{}, err
	}
	if closeErr != nil {
		return filespine.Slot{}, closeErr
	}
	return filespine.Slot{Kind: filespine.KindText, Text: string(data)}, nil
}

func destFile(decl filespine.File, targetBase string) (File, error) {
	mode := decl.Mode
	if mode == 0 {
		mode = 0o644
	}
	info := decl.Info
	if info == "" {
		info = "filespine:" + decl.Path
	}
	if decl.Type == filespine.TypeRef {
		var ref string
		for _, slot := range decl.Values {
			ref = slot.Ref
		}
		ft := TypeStatic
		if decl.Symlink {
			ft = TypeSymlink
		}
		return &StaticFile{
			BasicFile: BasicFile{
				RelPathStr:    filepath.FromSlash(decl.Path),
				TargetBaseDir: targetBase,
				FileMode:      mode,
				Info:          info,
				FileType:      ft,
				Module:        decl.Module,
			},
			AbsPath: ref,
		}, nil
	}
	data, err := filespine.Encode(decl)
	if err != nil {
		return nil, err
	}
	return &BufferFile{
		BasicFile: BasicFile{
			RelPathStr:    filepath.FromSlash(decl.Path),
			TargetBaseDir: targetBase,
			FileMode:      mode,
			Info:          info,
			FileType:      TypeStatic,
			Module:        decl.Module,
		},
		Content: data,
	}, nil
}
