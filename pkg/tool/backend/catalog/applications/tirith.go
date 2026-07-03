package apps

import (
	"context"
	"strings"

	"workspaced/pkg/modfile"
	"workspaced/pkg/tool/backend"
	"workspaced/pkg/tool/backend/catalog"
	"workspaced/pkg/tool/backend/github"
	"workspaced/pkg/tool/checks"
)

func init() {
	catalog.RegisterTool("tirith", newTirith)
}

type tirithTool struct {
	inner backend.Tool
}

func newTirith() (backend.Tool, error) {
	inner, err := github.NewTool("sheeki03/tirith")
	if err != nil {
		return nil, err
	}
	return tirithTool{inner: inner}, nil
}

func (t tirithTool) ListVersions(ctx context.Context) ([]string, error) {
	versions, err := t.inner.ListVersions(ctx)
	if err != nil {
		return nil, err
	}

	out := make([]string, 0, len(versions))
	for _, version := range versions {
		if isTirithProgramVersion(version) {
			out = append(out, version)
		}
	}
	return out, nil
}

func (t tirithTool) Install(ctx context.Context, version string, destDir string) error {
	return t.inner.Install(ctx, version, destDir)
}

func (t tirithTool) EnrichLockfile(entry *modfile.RenovateDependency) {
	t.inner.EnrichLockfile(entry)
	entry.Versioning = "semver"
}

func isTirithProgramVersion(version string) bool {
	v := strings.TrimPrefix(strings.TrimSpace(version), "v")
	parts := strings.Split(v, ".")
	if len(parts) != 3 {
		return false
	}
	for _, part := range parts {
		if part == "" {
			return false
		}
		for _, r := range part {
			if r < '0' || r > '9' {
				return false
			}
		}
	}
	return true
}

func (t tirithTool) InstallChecks() []checks.Check {
	return []checks.Check{checks.Binary("tirith")}
}
