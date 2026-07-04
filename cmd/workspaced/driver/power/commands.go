package power

import (
	"workspaced/pkg/driver/power"

	"github.com/spf13/cobra"
)

func init() {
	Registry.Add("lock", "Lock the session", power.Lock)
	Registry.Add("reboot", "Reboot the system", power.Reboot)
	Registry.Add("shutdown", "Power off the system", power.Shutdown)
	Registry.Add("suspend", "Suspend the system", power.Suspend)
	Registry.Register(func(parent *cobra.Command) {
		parent.AddCommand(&cobra.Command{
			Use:   "wake <host>",
			Short: "Send Wake-on-LAN magic packet",
			Args:  cobra.ExactArgs(1),
			RunE: func(c *cobra.Command, args []string) error {
				return power.Wake(c.Context(), args[0])
			},
		})
	})
}
