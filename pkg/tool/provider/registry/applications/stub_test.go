package apps

import (
	"context"

	"workspaced/pkg/modfile"
)

// stubTool is a minimal implementation of provider.Tool for use in tests,
// particularly for testing wrappers like tirithTool that decorate another Tool.
type stubTool struct {
	versions []string
}

func (t stubTool) ListVersions(context.Context) ([]string, error) {
	return append([]string(nil), t.versions...), nil
}

func (t stubTool) Install(context.Context, string, string) error {
	return nil
}

func (t stubTool) EnrichLockfile(*modfile.RenovateDependency) {}
