package lsp

import (
	"bytes"
	"encoding/json"
)

// mergeResults combines backend results. Array results are concatenated;
// otherwise the first non-null result in order wins.
func mergeResults(results []json.RawMessage) json.RawMessage {
	if len(results) == 0 {
		return json.RawMessage("null")
	}

	var arrays []json.RawMessage
	allArray := true
	for _, r := range results {
		if len(r) == 0 || string(r) == "null" {
			continue
		}
		trim := bytes.TrimSpace(r)
		if len(trim) == 0 || trim[0] != '[' {
			allArray = false
			break
		}
		arrays = append(arrays, r)
	}
	if allArray && len(arrays) > 0 {
		return concatJSONArrays(arrays)
	}

	// CompletionList-like: { isIncomplete, items: [] }
	if merged, ok := mergeCompletionLists(results); ok {
		return merged
	}

	for _, r := range results {
		if len(r) == 0 || string(r) == "null" {
			continue
		}
		return r
	}
	return json.RawMessage("null")
}

func concatJSONArrays(arrays []json.RawMessage) json.RawMessage {
	var out []json.RawMessage
	for _, a := range arrays {
		var items []json.RawMessage
		if err := json.Unmarshal(a, &items); err != nil {
			continue
		}
		out = append(out, items...)
	}
	if out == nil {
		return json.RawMessage("[]")
	}
	b, err := json.Marshal(out)
	if err != nil {
		return json.RawMessage("[]")
	}
	return b
}

func mergeCompletionLists(results []json.RawMessage) (json.RawMessage, bool) {
	type cl struct {
		IsIncomplete bool              `json:"isIncomplete"`
		Items        []json.RawMessage `json:"items"`
	}
	var lists []cl
	for _, r := range results {
		if len(r) == 0 || string(r) == "null" {
			continue
		}
		trim := bytes.TrimSpace(r)
		if len(trim) == 0 || trim[0] != '{' {
			return nil, false
		}
		var c cl
		if err := json.Unmarshal(r, &c); err != nil {
			return nil, false
		}
		// Heuristic: must have items key semantics
		var probe map[string]json.RawMessage
		if err := json.Unmarshal(r, &probe); err != nil {
			return nil, false
		}
		if _, ok := probe["items"]; !ok {
			return nil, false
		}
		lists = append(lists, c)
	}
	if len(lists) == 0 {
		return nil, false
	}
	var merged cl
	for _, l := range lists {
		if l.IsIncomplete {
			merged.IsIncomplete = true
		}
		merged.Items = append(merged.Items, l.Items...)
	}
	b, err := json.Marshal(merged)
	if err != nil {
		return nil, false
	}
	return b, true
}
