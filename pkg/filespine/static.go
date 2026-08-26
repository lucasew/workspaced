package filespine

import (
	"context"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"strings"
)

// StaticDir walks Root and emits ref slots for non-template files.
type StaticDir struct {
	Label string
	Root  string
}

func (d StaticDir) Name() string {
	if d.Label != "" {
		return d.Label
	}
	return "static:" + d.Root
}

func (d StaticDir) Provide(ctx context.Context) (Patch, error) {
	_ = ctx
	var slots []Contribution
	err := filepath.Walk(d.Root, func(p string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			if strings.HasSuffix(info.Name(), ".d.tmpl") {
				return filepath.SkipDir
			}
			return nil
		}
		rel, err := filepath.Rel(d.Root, p)
		if err != nil {
			return err
		}
		rel = filepath.ToSlash(rel)
		if IsTemplatePath(rel) {
			return nil
		}
		mode := info.Mode()
		if mode == 0 {
			mode = 0o644
		}
		slots = append(slots, Contribution{
			Path:    rel,
			Type:    TypeRef,
			Key:     "src",
			Slot:    Slot{Kind: KindRef, Ref: p},
			Mode:    mode.Perm(),
			Info:    fmt.Sprintf("static:%s", rel),
			Symlink: info.Mode()&os.ModeSymlink != 0,
		})
		return nil
	})
	if err != nil {
		return Patch{}, err
	}
	return Patch{Slots: slots}, nil
}

// IsTemplatePath is true for .tmpl files and .d.tmpl fragments.
func IsTemplatePath(rel string) bool {
	rel = filepath.ToSlash(rel)
	if strings.Contains(rel, ".d.tmpl/") || strings.HasSuffix(rel, ".d.tmpl") {
		return true
	}
	base := path.Base(rel)
	parts := strings.Split(base, ".")
	return (len(parts) >= 2 && parts[len(parts)-1] == "tmpl") ||
		(len(parts) >= 3 && parts[len(parts)-2] == "tmpl")
}
