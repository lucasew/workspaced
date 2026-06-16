package catalog

import (
	"errors"
	"fmt"
	"sort"
	"strings"

	"workspaced/pkg/tool"
	"workspaced/pkg/tool/backend"
)

var (
	ErrEmptyToolName = errors.New("curated tool name cannot be empty")
)

func init() {
	tool.Register("registry", &catalog{})
}

var namedTools = map[string]func() (backend.Tool, error){}

// RegisterTool registers a curated short-name tool (e.g. "uv", "tirith").
// These are looked up when the user writes a bare name (no "github:" or "mise:").
// The catalog backend is registered under the id "registry" for historical/compatibility reasons.
func RegisterTool(name string, f func() (backend.Tool, error)) {
	if _, ok := namedTools[name]; ok {
		panic(fmt.Sprintf("catalog: tool %s is being defined twice", name))
	}
	namedTools[name] = f
}

// catalog is the backend for short/curated tool names. It dispatches to
// concrete implementations (mostly GitHub-based with possible custom logic
// in the curated packages under catalog/applications).
type catalog struct{}

func (c *catalog) Name() string { return "Tool Catalog (curated short names)" }

func (c *catalog) Tool(ref string) (backend.Tool, error) {
	// Inline dispatch for named/curated tools.
	// See applications/ for the list of github-backed named tools.
	name := strings.TrimSpace(ref)
	if name == "" {
		return nil, ErrEmptyToolName
	}

	if ctor, ok := namedTools[name]; ok {
		return ctor()
	}

	return nil, fmt.Errorf("unknown named tool %q (the catalog only knows curated short names for github tools; bare names default to the catalog; use explicit 'mise:xxx' or 'github:owner/repo' for other tools)", name)
}

// NewTool constructs a Tool for a named entry in the catalog.
// It delegates to the catalog so the dispatch logic is not duplicated.
func NewTool(ref string) (backend.Tool, error) {
	// For direct construction of "registry:foo", we go through the same
	// named dispatch. This makes `catalog.NewTool("uv")` do the right thing.
	return (&catalog{}).Tool(ref)
}

// ListTools returns the sorted list of all known curated short names.
// These are the bare names (e.g. "uv", "ripgrep", "nodejs") usable without a
// provider prefix; they are served by the "registry" backend.
func ListTools() []string {
	names := make([]string, 0, len(namedTools))
	for n := range namedTools {
		names = append(names, n)
	}
	sort.Strings(names)
	return names
}
