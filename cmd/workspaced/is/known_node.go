package is

import (
	"fmt"
	"os"
	"github.com/lucasew/workspaced/internal/configcue"

	"github.com/spf13/cobra"
)

func init() {
	// known-node command that checks if current host is in config
	Registry.Register(func(parent *cobra.Command) {
		parent.AddCommand(&cobra.Command{
			Use:   "known-node",
			Short: "Check if current host is defined in config (by hostname or IP)",
			RunE: func(c *cobra.Command, args []string) error {
				cfg, err := configcue.LoadHome(c.Context())
				if err != nil {
					return fmt.Errorf("load config: %w", err)
				}
				var hosts map[string]struct {
					IPs []string `json:"ips"`
				}
				if err := cfg.Decode("hosts", &hosts); err != nil {
					return fmt.Errorf("decode hosts: %w", err)
				}

				// Try hostname first
				hostname, err := os.Hostname()
				if err == nil {
					if _, ok := hosts[hostname]; ok {
						return nil
					}
				}

				// Try matching by IP
				localIPs, err := getLocalIPs()
				if err != nil {
					return fmt.Errorf("get local IPs: %w", err)
				}
				localIPSet := make(map[string]struct{}, len(localIPs))
				for _, ip := range localIPs {
					localIPSet[ip] = struct{}{}
				}

				for nodeName, hostCfg := range hosts {
					for _, configIP := range hostCfg.IPs {
						if _, ok := localIPSet[configIP]; ok {
							return nil
						}
					}
					if nodeName == hostname {
						return nil
					}
				}

				return fmt.Errorf("current host not found in config (hostname: %s, IPs: %v)", hostname, localIPs)
			},
		})
	})
}
