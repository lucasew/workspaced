package mod

import (
	"github.com/spf13/cobra"
)

func init() {
	Registry.Register(func(c *cobra.Command) {
		c.AddCommand(&cobra.Command{
			Use:   "tidy",
			Short: "Alias for `mod lock`",
			RunE:  runModLock,
		})
	})
}
