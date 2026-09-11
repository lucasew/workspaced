package is

import (
	"context"
	"fmt"
	"net"
	"os"

	"github.com/lewtec/lewkit/x/cmd"
)

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

type Node struct {
	name cmd.StringArg
}

func (Node) Description() string { return "Check if current host matches the given node name" }
func (n *Node) Run(ctx context.Context) error {
	hostname, err := os.Hostname()
	if err != nil {
		return fmt.Errorf("get hostname: %w", err)
	}
	if hostname != n.name.Value() {
		return fmt.Errorf("current host '%s' is not '%s'", hostname, n.name.Value())
	}
	return nil
}
