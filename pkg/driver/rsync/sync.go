package rsync

import (
	"context"
	"io"

	"github.com/lucasew/workspaced/pkg/taskgroup"
)

// SyncWith validates paths, builds CLI args, and runs perform under RunWithTaskGroup.
func SyncWith(
	ctx context.Context,
	src, dst string,
	opts Options,
	modeArgs []string,
	perform func(ctx context.Context, args []string, st *taskgroup.Status, extraOut io.Writer) error,
) error {
	if err := ValidatePaths(src, dst); err != nil {
		return err
	}
	args := BuildCLIArgs(opts, modeArgs, src, dst)
	return RunWithTaskGroup(ctx, src, dst, opts, func(ctx context.Context, st *taskgroup.Status, extraOut io.Writer) error {
		return perform(ctx, args, st, extraOut)
	})
}
