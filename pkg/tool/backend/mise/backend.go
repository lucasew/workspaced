package mise

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"workspaced/pkg/miseutil"
	"workspaced/pkg/modfile"
	"workspaced/pkg/tool"
	"workspaced/pkg/tool/backend"
)

var (
	// ErrEmptyMiseSpec is returned when a mise spec is empty.
	ErrEmptyMiseSpec = errors.New("mise spec cannot be empty")
	// ErrEmptyMiseRef is returned when a mise ref is empty.
	ErrEmptyMiseRef = errors.New("mise ref cannot be empty")
	// ErrMissingMiseArtifactSpec is returned when a mise artifact spec is empty.
	ErrMissingMiseArtifactSpec = errors.New("missing mise artifact spec")
)

type Backend struct{}

func init() {
	tool.Register("mise", &Backend{})
}

func (p *Backend) Name() string { return "mise" }

// Tool returns a first-class Tool for the given mise ref (e.g. "node", "python").
func (p *Backend) Tool(ref string) (backend.Tool, error) {
	return NewTool(ref)
}

// ParsePackage kept for transitional compatibility.
func (p *Backend) ParsePackage(spec string) (backend.PackageConfig, error) {
	spec = strings.TrimSpace(spec)
	if spec == "" {
		return backend.PackageConfig{}, ErrEmptyMiseSpec
	}
	return backend.PackageConfig{
		Backend: "mise",
		Spec:    spec,
	}, nil
}

func (p *Backend) ListVersions(ctx context.Context, pkg backend.PackageConfig) ([]string, error) {
	version, err := miseutil.Latest(ctx, pkg.Spec)
	if err != nil {
		return nil, err
	}
	if version == "" {
		return nil, fmt.Errorf("mise latest returned empty version for %q", pkg.Spec)
	}
	return []string{version}, nil
}

func (p *Backend) GetArtifacts(ctx context.Context, pkg backend.PackageConfig, version string) ([]backend.Artifact, error) {
	_ = ctx
	return []backend.Artifact{{
		URL: pkg.Spec + "@" + strings.TrimSpace(version),
	}}, nil
}

func (p *Backend) Install(ctx context.Context, artifact backend.Artifact, destPath string) error {
	_ = destPath
	spec := strings.TrimSpace(artifact.URL)
	if spec == "" {
		return ErrMissingMiseArtifactSpec
	}
	return miseutil.Run(ctx, "install", spec)
}

func (p *Backend) EnsureBinary(ctx context.Context, pkg backend.PackageConfig, version string, cmdName string, destPath string) (string, error) {
	toolSpec := strings.TrimSpace(pkg.Spec) + "@" + strings.TrimSpace(version)

	binPath, err := miseutil.ResolveBinPath(ctx, cmdName, toolSpec)
	if err == nil {
		return ensureSymlink(destPath, binPath, cmdName)
	}

	if err := miseutil.Run(ctx, "install", toolSpec); err != nil {
		return "", err
	}

	binPath, err = miseutil.ResolveBinPath(ctx, cmdName, toolSpec)
	if err != nil {
		return "", err
	}
	return ensureSymlink(destPath, binPath, cmdName)
}

func ensureSymlink(destPath, binPath, cmdName string) (string, error) {
	if err := os.MkdirAll(destPath, 0755); err != nil {
		return "", err
	}

	linkPath := filepath.Join(destPath, "bin", cmdName)
	if err := os.MkdirAll(filepath.Dir(linkPath), 0755); err != nil {
		return "", err
	}

	if _, err := os.Lstat(linkPath); err == nil {
		if err := os.Remove(linkPath); err != nil {
			return "", err
		}
	}

	if err := os.Symlink(binPath, linkPath); err != nil {
		return "", err
	}

	return linkPath, nil
}

// ============================================================================
// MiseTool - exported Tool implementation for the mise backend
// ============================================================================

// MiseTool is the concrete Tool for packages managed via mise.
// Exported (with NewTool) for use by a future central registry backend.
type MiseTool struct {
	spec string
	p    *Backend
}

// NewTool constructs a MiseTool for the given ref (e.g. "node", "deno", "python@3.12").
// The ref is passed through to mise.
func NewTool(ref string) (backend.Tool, error) {
	ref = strings.TrimSpace(ref)
	if ref == "" {
		return nil, ErrEmptyMiseRef
	}
	return &MiseTool{spec: ref, p: &Backend{}}, nil
}

func (t *MiseTool) ListVersions(ctx context.Context) ([]string, error) {
	// mise backend's ListVersions currently only returns the single "latest"
	// resolved by the backend. We keep that behavior for the Tool.
	pkg := backend.PackageConfig{Spec: t.spec}
	return t.p.ListVersions(ctx, pkg)
}

func (t *MiseTool) Install(ctx context.Context, version string, destDir string) error {
	// For mise, Install on the Tool sets up the version and creates a
	// bin/ layout inside destDir pointing at the mise-managed binary.
	// We use the primary binary name derived from the spec when possible.
	// The richer EnsureBinary path is available via BinaryTool.
	toolSpec := t.spec
	if !strings.Contains(toolSpec, "@") && version != "" && version != "latest" {
		toolSpec = toolSpec + "@" + strings.TrimSpace(version)
	}

	// Delegate the actual mise install.
	art := backend.Artifact{URL: toolSpec} // the mise path reuses the old Install path
	if err := t.p.Install(ctx, art, destDir); err != nil {
		return err
	}

	// Best-effort: create a bin/ entry for the main command name (first word of spec)
	// This mirrors what the old manager expected after install.
	cmdName := t.spec
	if idx := strings.IndexAny(cmdName, "@:"); idx > 0 {
		cmdName = cmdName[:idx]
	}
	// Use EnsureBinary with the cmdName to populate destDir/bin/cmdName
	_, err := t.EnsureBinary(ctx, version, cmdName, destDir)
	return err
}

// BinaryTool implementation
func (t *MiseTool) EnsureBinary(ctx context.Context, version string, cmdName string, destDir string) (string, error) {
	pkg := backend.PackageConfig{Spec: t.spec}
	return t.p.EnsureBinary(ctx, pkg, version, cmdName, destDir)
}

// EnrichLockfile receives a pointer to the live lockfile dependency entry
// for this mise tool. See the interface godoc for why this by-reference
// design enables automatic migration of metadata when the Tool's logic
// evolves.
func (t *MiseTool) EnrichLockfile(entry *modfile.RenovateDependency) {
	// Mise is a frontend to many backends. Most do not have a single
	// renovate datasource. If this spec corresponds to something renovate
	// can manage, set the fields here.
	//
	// For now we leave renovate fields empty.
}
