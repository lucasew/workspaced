package filespine

import (
	"bytes"
	"encoding/json"
	"fmt"
	"slices"

	"cuelang.org/go/cue"
)

func decodeData(v cue.Value) (map[string]any, error) {
	if err := v.Err(); err != nil {
		return nil, err
	}
	if v.Kind() != cue.StructKind {
		return nil, fmt.Errorf("%w: got %s", ErrStructuredRoot, v.Kind())
	}
	raw, err := v.MarshalJSON()
	if err != nil {
		return nil, err
	}
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.UseNumber()
	var data map[string]any
	if err := dec.Decode(&data); err != nil {
		return nil, err
	}
	if data == nil {
		data = map[string]any{}
	}
	return normalizeJSON(data).(map[string]any), nil
}

func normalizeJSON(v any) any {
	switch x := v.(type) {
	case json.Number:
		if i, err := x.Int64(); err == nil {
			return i
		}
		f, err := x.Float64()
		if err != nil {
			return x.String()
		}
		return f
	case map[string]any:
		out := make(map[string]any, len(x))
		for k, vv := range x {
			out[k] = normalizeJSON(vv)
		}
		return out
	case []any:
		out := make([]any, len(x))
		for i, vv := range x {
			out[i] = normalizeJSON(vv)
		}
		return out
	default:
		return v
	}
}

func cloneData(in map[string]any) map[string]any {
	if in == nil {
		return nil
	}
	return normalizeJSON(in).(map[string]any)
}

func sortedKeys(m map[string]any) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	slices.Sort(keys)
	return keys
}
