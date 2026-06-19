package apps

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"

	"workspaced/pkg/modfile"
	"workspaced/pkg/semver"
	"workspaced/pkg/tool/backend"
	"workspaced/pkg/tool/backend/catalog"
	"workspaced/pkg/tool/backend/github"
	providerinstall "workspaced/pkg/tool/backend/install"
)

func init() {
	catalog.RegisterTool("ruby", newRuby)
}

type rubyTool struct {
	inner backend.Tool
}



func newRuby() (backend.Tool, error) {
	inner, err := github.NewTool("ruby/ruby-builder")
	if err != nil {
		return nil, err
	}
	return &rubyTool{inner: inner}, nil
}

func (t *rubyTool) ListVersions(ctx context.Context) ([]string, error) {
	vers, err := t.inner.ListVersions(ctx)
	if err != nil {
		return nil, err
	}

	out := []string{}
	seen := map[string]bool{}
	for _, v := range vers {
		if !strings.HasPrefix(v, "ruby-") {
			continue
		}
		ver := strings.TrimPrefix(v, "ruby-")
		if ver == "" || strings.Contains(ver, "-") || seen[ver] {
			continue
		}
		seen[ver] = true
		out = append(out, ver)
	}
	if len(out) == 0 {
		return nil, ErrNoVersions
	}

	// Sort descending semver so [0] == latest.
	svs := make(semver.SemVers, len(out))
	for i, s := range out {
		svs[i] = semver.Parse(s)
	}
	sort.Sort(sort.Reverse(svs))
	for i, s := range svs {
		out[i] = s.Original
	}
	return out, nil
}

func (t *rubyTool) Install(ctx context.Context, version string, destDir string) error {
	v := t.normalizeVersion(version)
	if v == "" || v == "latest" {
		vers, err := t.ListVersions(ctx)
		if err != nil {
			return err
		}
		if len(vers) == 0 {
			return ErrNoVersions
		}
		v = vers[0]
	}
	arts, err := t.ListArtifacts(ctx, v)
	if err != nil {
		return err
	}
	if len(arts) == 0 {
		return ErrNoPlatformArtifact
	}
	artifact := backend.SelectArtifact(arts, runtime.GOOS, runtime.GOARCH, "ruby")
	if artifact == nil {
		return fmt.Errorf("no suitable artifact found for %s/%s for registry:ruby@%s", runtime.GOOS, runtime.GOARCH, v)
	}
	return t.InstallArtifact(ctx, *artifact, destDir)
}

func (t *rubyTool) EnrichLockfile(entry *modfile.RenovateDependency) {
	entry.Provider = "registry"
	if strings.TrimSpace(entry.CurrentValue) == "" {
		entry.CurrentValue = entry.Version
	}
	entry.Versioning = "semver"
}

func (t *rubyTool) ListArtifacts(ctx context.Context, version string) ([]backend.Artifact, error) {
	v := t.normalizeVersion(version)
	if v == "" || v == "latest" {
		vers, err := t.ListVersions(ctx)
		if err != nil {
			return nil, err
		}
		if len(vers) == 0 {
			return nil, ErrNoVersions
		}
		v = vers[0]
	}

	tag := "ruby-" + v
	at, ok := t.inner.(backend.ArtifactTool)
	if !ok {
		return nil, fmt.Errorf("github tool does not implement ArtifactTool")
	}
	return at.ListArtifacts(ctx, tag)
}

func (t *rubyTool) InstallArtifact(ctx context.Context, artifact backend.Artifact, destDir string) error {
	return providerinstall.InstallArtifact(ctx, artifact, destDir, providerinstall.DownloadOptions{})
}

func (t *rubyTool) EnsureBinary(ctx context.Context, version string, cmdName string, destDir string) (string, error) {
	if err := t.Install(ctx, version, destDir); err != nil {
		return "", err
	}
	candidates := []string{
		filepath.Join(destDir, "bin", cmdName),
		filepath.Join(destDir, "bin", cmdName+".exe"),
		filepath.Join(destDir, cmdName),
		filepath.Join(destDir, cmdName+".exe"),
	}
	for _, c := range candidates {
		if _, err := os.Stat(c); err == nil {
			return c, nil
		}
	}
	return "", fmt.Errorf("binary %q not found in Ruby installation at %s", cmdName, destDir)
}

// --- helpers (as methods to avoid littering package scope) ---

func (t *rubyTool) normalizeVersion(version string) string {
	v := strings.TrimSpace(version)
	v = strings.TrimPrefix(v, "ruby-")
	v = strings.TrimPrefix(v, "Ruby-")
	if v == "" || v == "latest" {
		return v
	}
	return v
}



