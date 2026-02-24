package termux

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"time"
	"workspaced/pkg/driver"
	httpclientdriver "workspaced/pkg/driver/httpclient"
)

func init() {
	driver.Register[httpclientdriver.Driver](&Provider{})
}

type Provider struct{}

func (p *Provider) ID() string         { return "httpclient_termux" }
func (p *Provider) Name() string       { return "Termux HTTP Client" }
func (p *Provider) DefaultWeight() int { return 60 } // Higher than native

func (p *Provider) CheckCompatibility(ctx context.Context) error {
	if os.Getenv("TERMUX_VERSION") == "" {
		return fmt.Errorf("%w: not running in Termux", driver.ErrIncompatible)
	}
	return nil
}

func (p *Provider) New(ctx context.Context) (httpclientdriver.Driver, error) {
	return &Driver{}, nil
}

type Driver struct {
	once   sync.Once
	client *http.Client
}

func (d *Driver) Client() *http.Client {
	d.once.Do(func() {
		// Use Android's DNS resolver (8.8.8.8 as fallback)
		resolver := &net.Resolver{
			PreferGo: true,
			Dial: func(ctx context.Context, network, address string) (net.Conn, error) {
				// Try to read Android's DNS from properties
				// Fallback to Google DNS
				dnsServer := "8.8.8.8:53"

				// Try common Android DNS locations
				if dns := getAndroidDNS(); dns != "" {
					dnsServer = dns + ":53"
				}

				dialer := &net.Dialer{
					Timeout: 5 * time.Second,
				}
				return dialer.DialContext(ctx, "udp", dnsServer)
			},
		}

		// Custom dialer with our resolver
		dialer := &net.Dialer{
			Timeout:   30 * time.Second,
			KeepAlive: 30 * time.Second,
			Resolver:  resolver,
		}

		d.client = &http.Client{
			Transport: &http.Transport{
				DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
					// Force IPv4
					if network == "tcp" {
						network = "tcp4"
					}
					return dialer.DialContext(ctx, network, addr)
				},
				TLSClientConfig: &tls.Config{
					RootCAs: loadTermuxCerts(),
				},
				ForceAttemptHTTP2:     true,
				MaxIdleConns:          100,
				IdleConnTimeout:       90 * time.Second,
				TLSHandshakeTimeout:   10 * time.Second,
				ExpectContinueTimeout: 1 * time.Second,
			},
			Timeout: 60 * time.Second,
		}
	})
	return d.client
}

func getAndroidDNS() string {
	// Try to read from getprop (Android property)
	// Common DNS properties: net.dns1, net.dns2
	dnsServers := []string{
		"8.8.8.8", // Google DNS (fallback)
		"1.1.1.1", // Cloudflare DNS (fallback)
	}

	// Try reading from system properties if available
	if dns := os.Getenv("DNS_SERVER"); dns != "" {
		return dns
	}

	return dnsServers[0]
}

func loadTermuxCerts() *x509.CertPool {
	pool := x509.NewCertPool()

	// Termux-specific certificate locations
	certFiles := []string{
		"/data/data/com.termux/files/usr/etc/tls/cert.pem",
		"/data/data/com.termux/files/usr/etc/tls/certs/ca-certificates.crt",
		"/system/etc/security/cacerts",
	}

	// Try to load from each location
	for _, certFile := range certFiles {
		if certs, err := os.ReadFile(certFile); err == nil {
			if pool.AppendCertsFromPEM(certs) {
				return pool
			}
		}
	}

	// Try loading from directory (for Android)
	certDirs := []string{
		"/system/etc/security/cacerts",
		"/data/data/com.termux/files/usr/etc/tls/certs",
	}

	for _, certDir := range certDirs {
		entries, err := os.ReadDir(certDir)
		if err != nil {
			continue
		}
		loaded := 0
		for _, entry := range entries {
			if entry.IsDir() {
				continue
			}
			certPath := filepath.Join(certDir, entry.Name())
			if certs, err := os.ReadFile(certPath); err == nil {
				if pool.AppendCertsFromPEM(certs) {
					loaded++
				}
			}
		}
		if loaded > 0 {
			return pool
		}
	}

	// Last resort: return empty pool
	slog.Warn("could not load any CA certificates for Termux")
	return pool
}
