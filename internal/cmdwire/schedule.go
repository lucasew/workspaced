package cmdwire

import (
	"github.com/lucasew/workspaced/internal/cmdctx"
	"github.com/lucasew/workspaced/pkg/taskgroup"

	"github.com/spf13/cobra"
)

// ScheduleFunc wires plan/apply work into a task group and returns an AfterWait
// report printer.
type ScheduleFunc func(g *taskgroup.Group, cmd *cobra.Command, dryRun, showNoop bool) func() error

// RunAfterWait is the shared plan/apply RunE body: read --show-noop, optionally
// force dry-run (plan), schedule work, print the report after session wait.
func RunAfterWait(cmd *cobra.Command, forceDryRun bool, schedule ScheduleFunc) error {
	ctx := cmd.Context()
	showNoop, _ := cmd.Flags().GetBool("show-noop")
	dryRun := forceDryRun || cmdctx.IsDryRun(ctx)

	if forceDryRun {
		ctx = cmdctx.WithDryRun(ctx, true)
		cmd.SetContext(ctx)
		sess := taskgroup.MustSessionFrom(ctx)
		sess.Overlay(ctx)
		g := taskgroup.MustFromContext(ctx)
		sess.AfterWait(schedule(g, cmd, true, showNoop))
		return nil
	}

	g := taskgroup.MustFromContext(ctx)
	taskgroup.MustSessionFrom(ctx).AfterWait(schedule(g, cmd, dryRun, showNoop))
	return nil
}
