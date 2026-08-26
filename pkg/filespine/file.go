package filespine

import (
	"errors"
	"fmt"
	"io/fs"
	"maps"
	"path"
	"slices"
	"strings"

	"cuelang.org/go/cue"
)

const (
	TypeLines = "lines"
	TypeText  = "text"
	TypeRef   = "ref"
	TypeJSON  = "json"
	TypeTOML  = "toml"
	TypeYAML  = "yaml"
	TypeINI   = "ini"
	TypeXML   = "xml"
)

var (
	ErrUnknownFileType  = errors.New("unknown file type")
	ErrTypeConflict     = errors.New("file type conflict")
	ErrSlotConflict     = errors.New("slot conflict")
	ErrTextKeyCount     = errors.New("text file must have exactly one value")
	ErrRefKeyCount      = errors.New("ref file must have exactly one value")
	ErrRefSlotKind      = errors.New("ref file slot must be kind ref")
	ErrInvalidPath      = errors.New("invalid file path")
	ErrTargetConflict   = errors.New("target base conflict")
	ErrEmptyType        = errors.New("empty file type")
	ErrEmptyKey         = errors.New("empty value key")
	ErrEmptyPath        = errors.New("empty dest path")
	ErrStructuredRoot   = errors.New("structured file values must be a map")
	ErrINISection       = errors.New("ini values may nest one section map only")
	ErrXMLRoot          = errors.New("xml values must have exactly one root element")
	ErrXMLName          = errors.New("invalid xml name")
	ErrUnsupportedValue = errors.New("unsupported value")
)

// File is one dest path after unify.
type File struct {
	Path       string
	Type       string
	Values     map[string]Slot
	Data       map[string]any
	Mode       fs.FileMode
	Info       string
	Module     string
	TargetBase string
	Symlink    bool
}

func validPath(p string) error {
	p = strings.TrimSpace(p)
	if p == "" || p == "." || !fs.ValidPath(p) {
		return fmt.Errorf("%w: %q", ErrInvalidPath, p)
	}
	if path.IsAbs(p) || strings.HasPrefix(p, "~") {
		return fmt.Errorf("%w: %q", ErrInvalidPath, p)
	}
	return nil
}

func parseFile(name string, v cue.Value) (File, error) {
	if err := validPath(name); err != nil {
		return File{}, err
	}
	if err := v.Err(); err != nil {
		return File{}, fmt.Errorf("file %q: %w", name, err)
	}
	typ, err := v.LookupPath(cue.ParsePath("type")).String()
	if err != nil {
		return File{}, fmt.Errorf("file %q type: %w", name, err)
	}
	switch typ {
	case TypeLines, TypeText, TypeRef, TypeJSON, TypeTOML, TypeYAML, TypeINI, TypeXML:
	default:
		return File{}, fmt.Errorf("file %q: %w: %s", name, ErrUnknownFileType, typ)
	}
	valuesVal := v.LookupPath(cue.ParsePath("values"))
	if err := valuesVal.Err(); err != nil {
		return File{}, fmt.Errorf("file %q values: %w", name, err)
	}
	if isStructured(typ) {
		data, err := decodeData(valuesVal)
		if err != nil {
			return File{}, fmt.Errorf("file %q values: %w", name, err)
		}
		f := File{Path: name, Type: typ, Data: data, Mode: 0o644}
		if err := f.checkArity(); err != nil {
			return File{}, fmt.Errorf("file %q: %w", name, err)
		}
		return f, nil
	}
	iter, err := valuesVal.Fields()
	if err != nil {
		return File{}, fmt.Errorf("file %q values: %w", name, err)
	}
	values := make(map[string]Slot)
	for iter.Next() {
		key := iter.Selector().Unquoted()
		slot, err := parseSlot(iter.Value())
		if err != nil {
			return File{}, fmt.Errorf("file %q values.%s: %w", name, key, err)
		}
		values[key] = slot
	}
	f := File{Path: name, Type: typ, Values: values, Mode: 0o644}
	if err := f.checkArity(); err != nil {
		return File{}, fmt.Errorf("file %q: %w", name, err)
	}
	return f, nil
}

func isStructured(typ string) bool {
	switch typ {
	case TypeJSON, TypeTOML, TypeYAML, TypeINI, TypeXML:
		return true
	default:
		return false
	}
}

func (f File) checkArity() error {
	if isStructured(f.Type) {
		if f.Data == nil {
			return ErrStructuredRoot
		}
		return nil
	}
	n := len(f.Values)
	switch f.Type {
	case TypeText:
		if n != 1 {
			return fmt.Errorf("%w: got %d", ErrTextKeyCount, n)
		}
	case TypeRef:
		if n != 1 {
			return fmt.Errorf("%w: got %d", ErrRefKeyCount, n)
		}
		for _, slot := range f.Values {
			if slot.Kind != KindRef {
				return ErrRefSlotKind
			}
		}
	}
	return nil
}

func (f File) keys() []string {
	keys := slices.Collect(maps.Keys(f.Values))
	slices.Sort(keys)
	return keys
}
