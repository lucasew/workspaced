package filespine

import (
	"context"
	"fmt"
)

// Provider adds dest files to a compose.
type Provider interface {
	Name() string
	Provide(ctx context.Context) (Patch, error)
}

// Patch is one provider's dest additions.
// Files are full dest decls (CUE). Slots are keyed fragments (static dirs, templates).
type Patch struct {
	Files map[string]File
	Slots []Contribution
}

// Compose merges providers into one dest FS.
func Compose(ctx context.Context, providers ...Provider) (*FS, error) {
	declared := map[string]File{}
	var slots []Contribution
	for _, p := range providers {
		if p == nil {
			continue
		}
		patch, err := p.Provide(ctx)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", p.Name(), err)
		}
		declared, err = mergeFiles(declared, patch.Files)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", p.Name(), err)
		}
		slots = append(slots, patch.Slots...)
	}
	merged, err := Merge(declared, slots)
	if err != nil {
		return nil, err
	}
	return NewFS(merged), nil
}

func mergeFiles(a, b map[string]File) (map[string]File, error) {
	if len(b) == 0 {
		return a, nil
	}
	if len(a) == 0 {
		out := make(map[string]File, len(b))
		for k, f := range b {
			cp := f
			cp.Values = cloneValues(f.Values)
			cp.Data = cloneData(f.Data)
			out[k] = cp
		}
		return out, nil
	}
	out := make(map[string]File, len(a)+len(b))
	for k, f := range a {
		cp := f
		cp.Values = cloneValues(f.Values)
		cp.Data = cloneData(f.Data)
		out[k] = cp
	}
	for path, f := range b {
		cur, ok := out[path]
		if !ok {
			cp := f
			cp.Values = cloneValues(f.Values)
			cp.Data = cloneData(f.Data)
			out[path] = cp
			continue
		}
		if cur.Type != f.Type {
			return nil, fmt.Errorf("%w: %q: %s vs %s", ErrTypeConflict, path, cur.Type, f.Type)
		}
		if f.TargetBase != "" && cur.TargetBase != "" && f.TargetBase != cur.TargetBase {
			return nil, fmt.Errorf("%w: %q: %s vs %s", ErrTargetConflict, path, cur.TargetBase, f.TargetBase)
		}
		if cur.TargetBase == "" {
			cur.TargetBase = f.TargetBase
		}
		if f.Symlink {
			cur.Symlink = true
		}
		for k, slot := range f.Values {
			if existing, exists := cur.Values[k]; exists && !existing.equal(slot) {
				return nil, fmt.Errorf("%w: %q values.%s", ErrSlotConflict, path, k)
			}
			cur.Values[k] = slot
		}
		if cur.Data == nil && f.Data != nil {
			cur.Data = cloneData(f.Data)
		}
		if cur.Info == "" {
			cur.Info = f.Info
		}
		if cur.Module == "" {
			cur.Module = f.Module
		}
		out[path] = cur
	}
	return out, nil
}
