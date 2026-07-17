package demo

import (
	"github.com/spf13/cobra"
)

func init() {
	Registry.Register(func(parent *cobra.Command) {
		parent.AddCommand(&cobra.Command{
			Use:   "tasks",
			Short: "Run a set of tasks that demonstrate progress bars, logs, pools and dependencies",
			RunE: func(cmd *cobra.Command, args []string) error {
				return runTasksDemo(cmd)
			},
		})
	})
}
