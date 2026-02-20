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

			// Run dry-run apply
			applyCmd := execdriver.MustRun(ctx, "workspaced", "dispatch", "apply", "--dry-run")
			applyCmd.Stdout = os.Stdout
			applyCmd.Stderr = os.Stderr
			return applyCmd.Run()
		},
	}

	return cmd
}
