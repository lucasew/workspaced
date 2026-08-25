package filespine

import (
	"fmt"

	"cuelang.org/go/cue"
)

// Parse walks the workspaced.file CUE value (AST) into dest files.
// A missing or non-existent value is an empty map, not an error.
func Parse(fileValue cue.Value) (map[string]File, error) {
	out := map[string]File{}
	if !fileValue.Exists() {
		return out, nil
	}
	if err := fileValue.Err(); err != nil {
		return nil, fmt.Errorf("file: %w", err)
	}
	if fileValue.Kind() == cue.BottomKind && !fileValue.IsConcrete() {
		return out, nil
	}
	iter, err := fileValue.Fields()
	if err != nil {
		return nil, fmt.Errorf("file: %w", err)
	}
	for iter.Next() {
		name := iter.Selector().Unquoted()
		f, err := parseFile(name, iter.Value())
		if err != nil {
			return nil, err
		}
		out[name] = f
	}
	return out, nil
}

// LookupFile is workspaced.file on a workspaced root value.
func LookupFile(workspaced cue.Value) cue.Value {
	if !workspaced.Exists() {
		return cue.Value{}
	}
	return workspaced.LookupPath(cue.ParsePath("file"))
}
