package plan

import (
	"context"
	"os"

	execdriver "workspaced/pkg/driver/exec"
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

			g := taskgroup.MustFromContext(ctx)
			g.Go("home:plan", taskgroup.Control, func(ctx context.Context, s *taskgroup.Status) error {
				s.Update("planning changes")
				s.Progress(0, 1)
				defer s.Progress(1, 1)

				// Run dry-run apply
				applyArgs := []string{"home", "apply", "--dry-run"}
				if showNoop {
					applyArgs = append(applyArgs, "--show-noop")
				}
				applyCmd := execdriver.MustRun(ctx, "workspaced", applyArgs...)
				applyCmd.Stdout = os.Stdout
				applyCmd.Stderr = os.Stderr
				return applyCmd.Run()
			})

			return taskgroup.Run(g)
		},
	}

	cmd.Flags().Bool("show-noop", false, "Also show files that would not change")
	return cmd
}
