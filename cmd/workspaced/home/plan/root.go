package plan

import (
	"workspaced/cmd/workspaced/home/apply"
	"workspaced/pkg/cmdctx"
	"workspaced/pkg/taskgroup"

	"github.com/spf13/cobra"
)

func GetCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "plan",
		Short: "Show what would be applied (dry-run)",
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			showNoop, _ := cmd.Flags().GetBool("show-noop")

			// Force dry-run so that any code using cmdctx.IsDryRun(ctx) (or
			// the ApplyOptions inside the scheduled work) behaves as a plan.
			ctx = cmdctx.WithDryRun(ctx, true)
			cmd.SetContext(ctx)

			g := taskgroup.MustFromContext(ctx)
			apply.Schedule(g, cmd, true, showNoop)
			return taskgroup.Run(g)
		},
	}

	cmd.Flags().Bool("show-noop", false, "Also show files that would not change")
	return cmd
}
