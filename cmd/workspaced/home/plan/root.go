package plan

import (
	"github.com/lucasew/workspaced/cmd/workspaced/home/apply"
	"github.com/lucasew/workspaced/internal/cmdwire"

	"github.com/spf13/cobra"
)

func GetCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "plan",
		Short: "Show what would be applied (dry-run)",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return cmdwire.RunAfterWait(cmd, true, apply.Schedule)
		},
	}

	cmd.Flags().Bool("show-noop", false, "Also show files that would not change")
	return cmd
}
