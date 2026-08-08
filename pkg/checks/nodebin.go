package checks

import (
	"context"
	"os/exec"
	"path/filepath"

	execdriver "workspaced/pkg/driver/exec"
)

// NodeModuleBinRel returns the path of a node_modules/.bin entry relative to dir.
func NodeModuleBinRel(name string) string {
	return filepath.Join("node_modules", ".bin", name)
}

// RequireNodeModuleBin returns ErrNotApplicable when
// dir/node_modules/.bin/<name> is missing.
func RequireNodeModuleBin(dir, name string) error {
	return RequireFile(dir, NodeModuleBinRel(name))
}

// PrepareNodeModuleBin builds a command that runs dir/node_modules/.bin/<name>
// with args. Prefers node on PATH; falls back to `bun run --bun` so the script
// still runs when only Bun is installed. Returns ErrToolNotAvailable when
// neither runtime is present.
func PrepareNodeModuleBin(ctx context.Context, dir, name string, args ...string) (*exec.Cmd, error) {
	binPath := filepath.Join(dir, NodeModuleBinRel(name))
	if execdriver.IsBinaryAvailable(ctx, "node") {
		return execdriver.Run(ctx, binPath, args...)
	}
	if execdriver.IsBinaryAvailable(ctx, "bun") {
		bunArgs := append([]string{"run", "--bun", binPath}, args...)
		return execdriver.Run(ctx, "bun", bunArgs...)
	}

	return nil, ErrToolNotAvailable
}
