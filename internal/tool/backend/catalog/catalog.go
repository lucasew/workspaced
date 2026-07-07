package catalog

import (
	"context"
	"errors"
	"fmt"
	"runtime"
	"sort"
	"strings"

	"workspaced/internal/modfile"
	"workspaced/internal/tool"
	"workspaced/internal/tool/backend"
	"workspaced/internal/tool/backend/github"
	"workspaced/internal/tool/checks"
)

var (
	ErrEmptyToolName  = errors.New("curated tool name cannot be empty")
	errNoArtifactTool = errors.New("inner tool does not implement ArtifactTool")
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

// curatedGitHub is the standard adapter used by most github-backed entries
// in the tool catalog (the "registry"). It normalizes versions (strips
// leading "v") and install attempts (tries v-prefixed tag first) which is
// the common convention for curated short names.
type curatedGitHub struct {
	inner      backend.Tool
	binaryHint string
	checks     []checks.Check
}

func newCuratedGitHub(repo, binaryHint string, toolChecks ...checks.Check) (backend.Tool, error) {
	inner, err := github.NewTool(repo, binaryHint)
	if err != nil {
		return nil, err
	}
	return &curatedGitHub{inner: inner, binaryHint: binaryHint, checks: checks.Checks(toolChecks...)}, nil
}

func (t *curatedGitHub) ListVersions(ctx context.Context) ([]string, error) {
	vers, err := t.inner.ListVersions(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]string, 0, len(vers))
	for _, v := range vers {
		out = append(out, strings.TrimPrefix(strings.TrimSpace(v), "v"))
	}
	return out, nil
}

func (t *curatedGitHub) Install(ctx context.Context, version string, destDir string) error {
	v := strings.TrimSpace(version)
	try := func(ver string) error {
		if at, ok := t.inner.(backend.ArtifactTool); ok && t.binaryHint != "" {
			arts, err := at.ListArtifacts(ctx, ver)
			if err != nil {
				return err
			}
			if chosen := backend.SelectArtifact(arts, runtime.GOOS, runtime.GOARCH, t.binaryHint); chosen != nil {
				return at.InstallArtifact(ctx, *chosen, destDir)
			}
		}
		return t.inner.Install(ctx, ver, destDir)
	}
	if v == "" || v == "latest" {
		return try(v)
	}
	if !strings.HasPrefix(v, "v") {
		if err := try("v" + v); err == nil {
			return nil
		}
	}
	return try(v)
}

func (t *curatedGitHub) EnrichLockfile(entry *modfile.RenovateDependency) {
	t.inner.EnrichLockfile(entry)
}

func (t *curatedGitHub) InstallChecks() []checks.Check {
	return checks.Checks(t.checks...)
}

func (t *curatedGitHub) ListArtifacts(ctx context.Context, version string) ([]backend.Artifact, error) {
	at, ok := t.inner.(backend.ArtifactTool)
	if !ok {
		return nil, errNoArtifactTool
	}
	return at.ListArtifacts(ctx, version)
}

func (t *curatedGitHub) InstallArtifact(ctx context.Context, artifact backend.Artifact, destDir string) error {
	at, ok := t.inner.(backend.ArtifactTool)
	if !ok {
		return errNoArtifactTool
	}
	return at.InstallArtifact(ctx, artifact, destDir)
}

func (t *curatedGitHub) Fix(ctx context.Context, destDir string) error {
	fixer, ok := t.inner.(backend.InstallFixer)
	if !ok {
		return nil
	}
	return fixer.Fix(ctx, destDir)
}

// RegisterGitHub registers a simple github-backed curated tool. It uses the
// standard v-prefix handling used across the catalog for such entries.
// When toolChecks is empty, defaults to checks.Binary(name).
// Artifact selection is biased toward the on-disk binary name derived from
// the first checks.Binary(...) entry, falling back to name.
func RegisterGitHub(name, repo string, toolChecks ...checks.Check) {
	if len(toolChecks) == 0 {
		toolChecks = checks.Checks(checks.Binary(name))
	}
	checksCopy := checks.Checks(toolChecks...)
	hint := binaryHintFromChecks(name, checksCopy)
	RegisterTool(name, func() (backend.Tool, error) {
		return newCuratedGitHub(repo, hint, checksCopy...)
	})
}

func binaryHintFromChecks(fallback string, list []checks.Check) string {
	for _, c := range list {
		if name := c.Name(); strings.HasPrefix(name, "binary:") {
			if hint := strings.TrimPrefix(name, "binary:"); hint != "" {
				return hint
			}
		}
	}
	return fallback
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
