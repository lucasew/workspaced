// Package tool is the public library surface for ensuring and resolving tool
// binaries (same ensure path as `workspaced tool which` / `tool with`).
package tool

import (
	"context"

	itool "github.com/lucasew/workspaced/internal/tool"
)

// EnsureInstalled ensures toolSpec is on disk (installing if needed) and returns
// the absolute path to binary inside that install. Same semantics as the CLI:
//
//	workspaced tool which <tool-spec> <binary>
//
// Import package prelude (or otherwise register tool backends) before calling,
// or use EnsureInstalledWithBackends which loads the standard registry set.
func EnsureInstalled(ctx context.Context, toolSpec, binary string) (string, error) {
	m, err := itool.NewManager()
	if err != nil {
		return "", err
	}
	return m.EnsureInstalled(ctx, toolSpec, binary)
}
