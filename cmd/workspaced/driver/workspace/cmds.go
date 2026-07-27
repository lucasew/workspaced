package workspace

import (
	"github.com/lucasew/workspaced/pkg/driver/wm"

	"github.com/spf13/cobra"
)

func init() {
	Registry.Register(func(parent *cobra.Command) {
		parent.AddCommand(&cobra.Command{
			Use:   "rotate",
			Short: "Rotate workspaces across outputs",
			RunE: func(c *cobra.Command, args []string) error {
				return wm.RotateWorkspaces(c.Context())
			},
		})
		parent.AddCommand(&cobra.Command{
			Use:   "scratchpad",
			Short: "Toggle scratchpad visibility with status info",
			RunE: func(c *cobra.Command, args []string) error {
				return wm.ToggleScratchpadWithInfo(c.Context())
			},
		})
		parent.AddCommand(&cobra.Command{
			Use:   "next",
			Short: "Go to the next available workspace",
			RunE: func(c *cobra.Command, args []string) error {
				move, _ := c.Flags().GetBool("move")
				return wm.NextWorkspace(c.Context(), move)
			},
		})
	})
}
