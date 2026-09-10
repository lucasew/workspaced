package filespine

import (
	"fmt"

	"cuelang.org/go/cue"
)

const (
	ModeHome     = "home"
	ModeCodebase = "codebase"
	ModeSystem   = "system"
)

// NamespaceBase is the dest root for a preset folder. home, codebase, and
// system have no fixed path; the apply command fills those in.
var NamespaceBase = map[string]string{
	"etc":  "/etc",
	"usr":  "/usr",
	"root": "/",
	"var":  "/var",
	"bin":  "/usr/local/bin",
}

// IsNamespace reports whether name is a reserved dest tree (module preset or system).
func IsNamespace(name string) bool {
	switch name {
	case "home", "codebase", "etc", "usr", "root", "var", "bin", "system":
		return true
	default:
		return false
	}
}

// ParseRootOptions selects which dest trees ParseRoot emits.
type ParseRootOptions struct {
	// Mode is home, codebase, or system. Empty means home.
	Mode string
}

// Parse walks the dest CUE value into files. Flat keys are home dests.
func Parse(fileValue cue.Value) (map[string]File, error) {
	return ParseRoot(fileValue, ParseRootOptions{Mode: ModeHome})
}

// ParseRoot walks a #Root dest value. Flat keys and file.home are home.
// file.codebase is the repo tree. Other preset names match module folders.
func ParseRoot(fileValue cue.Value, opts ParseRootOptions) (map[string]File, error) {
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
	mode := opts.Mode
	if mode == "" {
		mode = ModeHome
	}
	for iter.Next() {
		name := iter.Selector().Unquoted()
		if IsNamespace(name) {
			if !NamespaceVisible(mode, name) {
				continue
			}
			tree, err := parseTree(name, iter.Value())
			if err != nil {
				return nil, err
			}
			base := NamespaceBase[name]
			for path, f := range tree {
				f.TargetBase = base
				out[path] = f
			}
			continue
		}
		if mode != ModeHome {
			continue
		}
		f, err := parseFile(name, iter.Value())
		if err != nil {
			return nil, err
		}
		out[name] = f
	}
	return out, nil
}

func parseTree(ns string, v cue.Value) (map[string]File, error) {
	out := map[string]File{}
	if !v.Exists() {
		return out, nil
	}
	if err := v.Err(); err != nil {
		return nil, fmt.Errorf("file.%s: %w", ns, err)
	}
	if v.Kind() == cue.BottomKind && !v.IsConcrete() {
		return out, nil
	}
	iter, err := v.Fields()
	if err != nil {
		return nil, fmt.Errorf("file.%s: %w", ns, err)
	}
	for iter.Next() {
		name := iter.Selector().Unquoted()
		f, err := parseFile(name, iter.Value())
		if err != nil {
			return nil, fmt.Errorf("file.%s: %w", ns, err)
		}
		out[name] = f
	}
	return out, nil
}

// NamespaceVisible reports whether dest namespace ns is emitted for mode.
func NamespaceVisible(mode, ns string) bool {
	switch mode {
	case ModeCodebase:
		return ns == "codebase"
	case ModeSystem:
		return ns == "system"
	default:
		return ns != "codebase" && ns != "system"
	}
}
