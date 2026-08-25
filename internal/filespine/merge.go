package filespine

import (
	"fmt"
	"io/fs"
)

// Contribution is one keyed slot for a dest path. Module scan emits these.
type Contribution struct {
	Path       string
	Type       string
	Key        string
	Slot       Slot
	Mode       fs.FileMode
	Info       string
	Module     string
	TargetBase string
	Symlink    bool
}

// Merge unifies declared CUE files with lowered contributions.
// Same path and different type is an error. Same key must be identical.
func Merge(declared map[string]File, extras []Contribution) (map[string]File, error) {
	out := make(map[string]File, len(declared)+len(extras))
	for path, f := range declared {
		cp := f
		cp.Values = cloneValues(f.Values)
		cp.Data = cloneData(f.Data)
		if cp.Mode == 0 {
			cp.Mode = 0o644
		}
		out[path] = cp
	}
	for _, c := range extras {
		if err := validPath(c.Path); err != nil {
			return nil, err
		}
		if c.Type == "" {
			return nil, fmt.Errorf("file %q: %w", c.Path, ErrEmptyType)
		}
		if c.Key == "" {
			return nil, fmt.Errorf("file %q: %w", c.Path, ErrEmptyKey)
		}
		cur, ok := out[c.Path]
		if !ok {
			mode := c.Mode
			if mode == 0 {
				mode = 0o644
			}
			out[c.Path] = File{
				Path:       c.Path,
				Type:       c.Type,
				Values:     map[string]Slot{c.Key: c.Slot},
				Mode:       mode,
				Info:       c.Info,
				Module:     c.Module,
				TargetBase: c.TargetBase,
				Symlink:    c.Symlink,
			}
			continue
		}
		if cur.Type != c.Type {
			return nil, fmt.Errorf("%w: %q: %s vs %s", ErrTypeConflict, c.Path, cur.Type, c.Type)
		}
		if c.TargetBase != "" && cur.TargetBase != "" && c.TargetBase != cur.TargetBase {
			return nil, fmt.Errorf("%w: %q: %s vs %s", ErrTargetConflict, c.Path, cur.TargetBase, c.TargetBase)
		}
		if cur.TargetBase == "" {
			cur.TargetBase = c.TargetBase
		}
		if c.Symlink {
			cur.Symlink = true
		}
		if existing, exists := cur.Values[c.Key]; exists && !existing.equal(c.Slot) {
			return nil, fmt.Errorf("%w: %q values.%s", ErrSlotConflict, c.Path, c.Key)
		}
		cur.Values[c.Key] = c.Slot
		if cur.Info == "" {
			cur.Info = c.Info
		}
		if cur.Module == "" {
			cur.Module = c.Module
		}
		out[c.Path] = cur
	}
	for path, f := range out {
		if err := f.checkArity(); err != nil {
			return nil, fmt.Errorf("file %q: %w", path, err)
		}
	}
	return out, nil
}

func cloneValues(in map[string]Slot) map[string]Slot {
	if in == nil {
		return map[string]Slot{}
	}
	out := make(map[string]Slot, len(in))
	for k, v := range in {
		out[k] = v
	}
	return out
}
