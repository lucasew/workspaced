package modulecue

import (
	"encoding/json"
	"fmt"

	"cuelang.org/go/cue"
	"cuelang.org/go/cue/cuecontext"
)

// EvalFile evaluates module.file with the given workspaced root as context.
// ok is false when the module has no file map.
func EvalFile(modPath string, root map[string]any) (json.RawMessage, bool, error) {
	ctx := cuecontext.New()
	v, err := compileModuleWithContext(ctx, modPath, root)
	if err != nil {
		return nil, false, err
	}
	fileVal := v.LookupPath(cue.ParsePath("module.file"))
	if !fileVal.Exists() {
		return nil, false, nil
	}
	if err := fileVal.Err(); err != nil {
		return nil, false, fmt.Errorf("lookup module.file in %s: %w", FilePath(modPath), err)
	}
	if fileVal.Kind() != cue.StructKind {
		return nil, false, fmt.Errorf("module.file in %s: %w: got %s", FilePath(modPath), ErrFileNotStruct, fileVal.Kind())
	}
	b, err := fileVal.MarshalJSON()
	if err != nil {
		return nil, false, fmt.Errorf("marshal module.file in %s: %w", FilePath(modPath), err)
	}
	if len(b) == 0 || string(b) == "null" || string(b) == "{}" {
		return nil, false, nil
	}
	return b, true, nil
}
