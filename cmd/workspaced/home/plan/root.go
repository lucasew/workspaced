package plan

import (
	"github.com/lucasew/workspaced/cmd/workspaced/home/apply"
	"github.com/lucasew/workspaced/internal/cmdctx"
	"github.com/lucasew/workspaced/pkg/taskgroup"

	"github.com/spf13/cobra"
)

func GetCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "plan",
		Short: "Show what would be applied (dry-run)",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			showNoop, _ := cmd.Flags().GetBool("show-noop")

			// Force dry-run so that any code using cmdctx.IsDryRun(ctx) (or
			// the ApplyOptions inside the scheduled work) behaves as a plan.
			// Overlay so task contexts (built from the Enter-time group ctx)
			// see dry-run too — needed for --no-cache materializer skips.
			ctx = cmdctx.WithDryRun(ctx, true)
			cmd.SetContext(ctx)
			sess := taskgroup.MustSessionFrom(ctx)
			sess.Overlay(ctx)

			g := taskgroup.MustFromContext(ctx)
			printReport := apply.Schedule(g, cmd, true, showNoop)
			sess.AfterWait(printReport)
			return nil
		},
	}

	cmd.Flags().Bool("show-noop", false, "Also show files that would not change")
	return cmd
}
