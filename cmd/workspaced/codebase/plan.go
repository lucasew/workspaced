package codebase

import (
	"github.com/lucasew/workspaced/internal/cmdctx"
	"github.com/lucasew/workspaced/pkg/taskgroup"

	"github.com/spf13/cobra"
)

func init() {
	Registry.Register(func(parent *cobra.Command) {
		cmd := &cobra.Command{
			Use:   "plan",
			Short: "Show what would be applied to the repo root (dry-run)",
			Args:  cobra.NoArgs,
			RunE: func(cmd *cobra.Command, args []string) error {
				ctx := cmd.Context()
				showNoop, _ := cmd.Flags().GetBool("show-noop")

				// Force dry-run on apply options and task contexts (Overlay).
				ctx = cmdctx.WithDryRun(ctx, true)
				cmd.SetContext(ctx)
				sess := taskgroup.MustSessionFrom(ctx)
				sess.Overlay(ctx)

				g := taskgroup.MustFromContext(ctx)
				printReport := Schedule(g, cmd, true, showNoop)
				sess.AfterWait(printReport)
				return nil
			},
		}
		cmd.Flags().Bool("show-noop", false, "Also show files that would not change")
		parent.AddCommand(cmd)
	})
}
