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
	catalog.RegisterTool("actionlint", newActionlint)
}

type actionlintTool struct {
	inner backend.Tool
}

func newActionlint() (backend.Tool, error) {
	inner, err := github.NewTool("rhysd/actionlint")
	if err != nil {
		return nil, err
	}
	return &actionlintTool{inner: inner}, nil
}

func (t *actionlintTool) ListVersions(ctx context.Context) ([]string, error) {
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

func (t *actionlintTool) Install(ctx context.Context, version string, destDir string) error {
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

func (t *actionlintTool) EnrichLockfile(entry *modfile.RenovateDependency) {
	t.inner.EnrichLockfile(entry)
}
