package is

import (
	"fmt"
	"net"
	"os"
	"slices"
	"workspaced/pkg/config"

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

	// Add 'known-node' command that checks if current host is in config
	Registry.Register(func(parent *cobra.Command) {
		parent.AddCommand(&cobra.Command{
			Use:   "known-node",
			Short: "Check if current host is defined in config (by hostname or IP)",
			RunE: func(c *cobra.Command, args []string) error {
				cfg, err := config.Load()
				if err != nil {
					return fmt.Errorf("failed to load config: %w", err)
				}

				// Try hostname first
				hostname, err := os.Hostname()
				if err == nil {
					if _, ok := cfg.Hosts[hostname]; ok {
						return nil
					}
				}

				// Try matching by IP
				localIPs, err := getLocalIPs()
				if err != nil {
					return fmt.Errorf("failed to get local IPs: %w", err)
				}

				for nodeName, hostCfg := range cfg.Hosts {
					for _, configIP := range hostCfg.IPs {
						if slices.Contains(localIPs, configIP) {
							// Found match by IP
							return nil
						}
					}
					// Check hostname as fallback
					if nodeName == hostname {
						return nil
					}
				}

				return fmt.Errorf("current host not found in config (hostname: %s, IPs: %v)", hostname, localIPs)
			},
		})
	})
}
