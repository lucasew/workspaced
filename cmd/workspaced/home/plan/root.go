package plan

import (
	"os"
	execdriver "workspaced/pkg/driver/exec"

	"github.com/spf13/cobra"
)

func GetCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "plan",
		Short: "Show what would be applied (dry-run)",
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			showNoop, _ := cmd.Flags().GetBool("show-noop")

			// Run dry-run apply
			applyArgs := []string{"home", "apply", "--dry-run"}
			if showNoop {
				applyArgs = append(applyArgs, "--show-noop")
			}
			applyCmd := execdriver.MustRun(ctx, "workspaced", applyArgs...)
			applyCmd.Stdout = os.Stdout
			applyCmd.Stderr = os.Stderr
			return applyCmd.Run()
		},
	}

	cmd.Flags().Bool("show-noop", false, "Also show files that would not change")
	return cmd
}
