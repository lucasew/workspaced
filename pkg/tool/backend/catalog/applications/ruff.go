package apps

import (
	"context"
	"strings"

	"workspaced/pkg/modfile"
	"workspaced/pkg/tool/backend"
	"workspaced/pkg/tool/backend/catalog"
	"workspaced/pkg/tool/backend/github"
)

func init() {
	catalog.RegisterTool("ruff", newRuff)
}

type ruffTool struct {
	inner backend.Tool
}

func newRuff() (backend.Tool, error) {
	inner, err := github.NewTool("astral-sh/ruff")
	if err != nil {
		return nil, err
	}
	return &ruffTool{inner: inner}, nil
}

func (t *ruffTool) ListVersions(ctx context.Context) ([]string, error) {
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

func (t *ruffTool) Install(ctx context.Context, version string, destDir string) error {
	v := strings.TrimSpace(version)
	if v == "" || v == "latest" {
		return t.inner.Install(ctx, v, destDir)
	}
	if !strings.HasPrefix(v, "v") {
		if err := t.inner.Install(ctx, "v"+v, destDir); err == nil {
			return nil
		}
	}
	return t.inner.Install(ctx, v, destDir)
}

func (t *ruffTool) EnrichLockfile(entry *modfile.RenovateDependency) {
	t.inner.EnrichLockfile(entry)
}
