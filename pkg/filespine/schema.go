package filespine

import (
	_ "embed"
	"errors"
	"fmt"
	"strings"

	"cuelang.org/go/cue"
)

//go:embed file.cue
var FileCUE string

var (
	ErrEmptyMountPath   = errors.New("empty filespine mount path")
	ErrInvalidMountPath = errors.New("invalid filespine mount path")
	ErrNilCueContext    = errors.New("nil cue context")
)

// Mount returns CUE that defines the dest schema and constrains path to #Tree.
// path is dotted, e.g. "workspaced.file" or "app.dest".
func Mount(path string) (string, error) {
	parts, err := splitCuePath(path)
	if err != nil {
		return "", err
	}
	var b strings.Builder
	b.WriteString("_filespine: {\n")
	b.WriteString(FileCUE)
	if !strings.HasSuffix(FileCUE, "\n") {
		b.WriteByte('\n')
	}
	b.WriteString("}\n")
	for i, p := range parts {
		b.WriteString(strings.Repeat("\t", i))
		if i == len(parts)-1 {
			b.WriteString(p)
			b.WriteString("?: _filespine.#Tree\n")
			continue
		}
		b.WriteString(p)
		b.WriteString(": {\n")
	}
	for i := len(parts) - 2; i >= 0; i-- {
		b.WriteString(strings.Repeat("\t", i))
		b.WriteString("}\n")
	}
	return b.String(), nil
}

// Constrain unifies Mount(path) onto v.
func Constrain(v cue.Value, path string) (cue.Value, error) {
	src, err := Mount(path)
	if err != nil {
		return cue.Value{}, err
	}
	ctx := v.Context()
	if ctx == nil {
		return cue.Value{}, fmt.Errorf("filespine constrain %s: %w", path, ErrNilCueContext)
	}
	layer := ctx.CompileString(src, cue.Filename("filespine.cue"))
	if err := layer.Err(); err != nil {
		return cue.Value{}, fmt.Errorf("compile filespine mount %s: %w", path, err)
	}
	out := v.Unify(layer)
	if err := out.Err(); err != nil {
		return cue.Value{}, fmt.Errorf("unify filespine mount %s: %w", path, err)
	}
	return out, nil
}

// ParseAt walks root at path (dotted) into dest files.
func ParseAt(root cue.Value, path string) (map[string]File, error) {
	if strings.TrimSpace(path) == "" {
		return Parse(root)
	}
	return Parse(root.LookupPath(cue.ParsePath(path)))
}

func splitCuePath(path string) ([]string, error) {
	path = strings.TrimSpace(path)
	if path == "" {
		return nil, ErrEmptyMountPath
	}
	parts := strings.Split(path, ".")
	for _, p := range parts {
		if p == "" || !isCueIdent(p) {
			return nil, fmt.Errorf("%w: %q", ErrInvalidMountPath, path)
		}
	}
	return parts, nil
}

func isCueIdent(s string) bool {
	if s == "" {
		return false
	}
	for i, r := range s {
		if i == 0 {
			if r != '_' && (r < 'A' || r > 'Z') && (r < 'a' || r > 'z') {
				return false
			}
			continue
		}
		if r != '_' && (r < 'A' || r > 'Z') && (r < 'a' || r > 'z') && (r < '0' || r > '9') {
			return false
		}
	}
	return true
}
