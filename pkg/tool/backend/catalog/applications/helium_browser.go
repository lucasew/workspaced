package apps

import (
	"context"
	"fmt"
	"runtime"
	"sort"
	"strings"

	"workspaced/pkg/modfile"
	"workspaced/pkg/semver"
	"workspaced/pkg/tool/backend"
	"workspaced/pkg/tool/backend/catalog"
	"workspaced/pkg/tool/backend/github"
	providerinstall "workspaced/pkg/tool/backend/install"
	"workspaced/pkg/tool/checks"
)

func init() {
	catalog.RegisterTool("helium-browser", newHeliumBrowser)
}

type heliumBrowserTool struct {
	tools []backend.Tool
}

func newHeliumBrowser() (backend.Tool, error) {
	repoNames := []string{
		"imputnet/helium-linux",
		"imputnet/helium-macos",
		"imputnet/helium-windows",
	}
	tools := make([]backend.Tool, 0, len(repoNames))
	for _, r := range repoNames {
		t, err := github.NewTool(r)
		if err != nil {
			return nil, err
		}
		tools = append(tools, t)
	}
	return &heliumBrowserTool{tools: tools}, nil
}

func (t *heliumBrowserTool) ListVersions(ctx context.Context) ([]string, error) {
	seen := make(map[string]struct{})
	var collected []string
	for _, inner := range t.tools {
		vers, err := inner.ListVersions(ctx)
		if err != nil {
			// One platform repo may be empty or temporarily unavailable; keep going.
			continue
		}
		for _, v := range vers {
			v = strings.TrimSpace(v)
			if v == "" {
				continue
			}
			if _, ok := seen[v]; ok {
				continue
			}
			seen[v] = struct{}{}
			collected = append(collected, v)
		}
	}
	if len(collected) == 0 {
		return nil, ErrNoVersions
	}

	// Sort descending (newest first) using the project's semver helper.
	svs := make(semver.SemVers, len(collected))
	for i, s := range collected {
		svs[i] = semver.Parse(s)
	}
	sort.Sort(sort.Reverse(svs))

	out := make([]string, len(svs))
	for i, s := range svs {
		out[i] = s.Original
	}
	return out, nil
}

func (t *heliumBrowserTool) Install(ctx context.Context, version string, destDir string) error {
	v := strings.TrimSpace(version)
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

	artifact := backend.SelectArtifact(arts, runtime.GOOS, runtime.GOARCH, "helium")
	if artifact == nil {
		return fmt.Errorf("no suitable artifact found for %s/%s for registry:helium-browser@%s", runtime.GOOS, runtime.GOARCH, v)
	}

	return t.InstallArtifact(ctx, *artifact, destDir)
}

func (t *heliumBrowserTool) EnrichLockfile(entry *modfile.RenovateDependency) {
	entry.Versioning = "semver"
}

func (t *heliumBrowserTool) ListArtifacts(ctx context.Context, version string) ([]backend.Artifact, error) {
	v := strings.TrimSpace(version)
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

	var all []backend.Artifact
	for _, inner := range t.tools {
		at, ok := inner.(backend.ArtifactTool)
		if !ok {
			continue
		}
		arts, err := at.ListArtifacts(ctx, v)
		if err != nil {
			// The version tag may not exist in this OS-specific repo.
			continue
		}
		all = append(all, arts...)
	}

	if len(all) == 0 {
		return nil, ErrNoPlatformArtifact
	}
	return all, nil
}

func (t *heliumBrowserTool) InstallArtifact(ctx context.Context, artifact backend.Artifact, destDir string) error {
	return providerinstall.InstallArtifact(ctx, artifact, destDir, providerinstall.DownloadOptions{})
}

func (t *heliumBrowserTool) InstallChecks() []checks.Check {
	return checks.Checks(checks.Binary("helium"))
}
