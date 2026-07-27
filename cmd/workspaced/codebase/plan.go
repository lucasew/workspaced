package codebase

import (
	"github.com/lucasew/workspaced/internal/cmdwire"

	"github.com/spf13/cobra"
)

func init() {
	Registry.Register(func(parent *cobra.Command) {
		cmd := &cobra.Command{
			Use:   "plan",
			Short: "Show what would be applied to the repo root (dry-run)",
			Args:  cobra.NoArgs,
			RunE: func(cmd *cobra.Command, args []string) error {
				return cmdwire.RunAfterWait(cmd, true, Schedule)
			},
		}
		cmd.Flags().Bool("show-noop", false, "Also show files that would not change")
		parent.AddCommand(cmd)
	})
}
