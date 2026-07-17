package is

import (
	"fmt"
	"net"
	"os"

	"github.com/spf13/cobra"
)

// getLocalIPs returns all non-loopback IPv4 and IPv6 addresses
func getLocalIPs() ([]string, error) {
	addrs, err := net.InterfaceAddrs()
	if err != nil {
		return nil, err
	}

	var ips []string
	for _, addr := range addrs {
		if ipnet, ok := addr.(*net.IPNet); ok && !ipnet.IP.IsLoopback() {
			ips = append(ips, ipnet.IP.String())
		}
	}
	return ips, nil
}

func init() {
	Registry.Register(func(parent *cobra.Command) {
		parent.AddCommand(&cobra.Command{
			Use:   "node <name>",
			Short: "Check if current host matches the given node name",
			Args:  cobra.ExactArgs(1),
			RunE: func(c *cobra.Command, args []string) error {
				hostname, err := os.Hostname()
				if err != nil {
					return fmt.Errorf("failed to get hostname: %w", err)
				}

				nodeName := args[0]
				if hostname != nodeName {
					return fmt.Errorf("current host '%s' is not '%s'", hostname, nodeName)
				}

				return nil
			},
		})
	})
}
