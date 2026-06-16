package termux

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"time"

	"workspaced/pkg/driver"
	httpclientdriver "workspaced/pkg/driver/httpclient"
	"workspaced/pkg/logging"
)

func init() {
	driver.Register[httpclientdriver.Driver](&Factory{})
}

type Factory struct{}

func (p *Factory) ID() string   { return "httpclient_termux" }
func (p *Factory) Name() string { return "Termux HTTP Client" }

func (p *Factory) CheckCompatibility(ctx context.Context) error {
	if os.Getenv("TERMUX_VERSION") == "" {
		return fmt.Errorf("%w: not running in Termux", driver.ErrIncompatible)
	}
	return nil
}

func (p *Factory) New(ctx context.Context) (httpclientdriver.Driver, error) {
	return &Driver{
		rootCAs: loadTermuxCerts(ctx),
	}, nil
}

type Driver struct {
	once    sync.Once
	client  *http.Client
	rootCAs *x509.CertPool
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

		innerTransport := &http.Transport{
			DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
				// Force IPv4
				if network == "tcp" {
					network = "tcp4"
				}
				return dialer.DialContext(ctx, network, addr)
			},
			TLSClientConfig: &tls.Config{
				RootCAs: d.rootCAs,
			},
			ForceAttemptHTTP2:     true,
			MaxIdleConns:          100,
			IdleConnTimeout:       90 * time.Second,
			TLSHandshakeTimeout:   10 * time.Second,
			ExpectContinueTimeout: 1 * time.Second,
		}

		d.client = &http.Client{
			Transport: httpclientdriver.WithProgress(innerTransport),
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

func loadTermuxCerts(ctx context.Context) *x509.CertPool {
	pool := x509.NewCertPool()

	// Termux-specific certificate locations
	certFiles := []string{
		"/data/data/com.termux/files/usr/etc/tls/cert.pem",
		"/data/data/com.termux/files/usr/etc/tls/certs/ca-certificates.crt",
		"/system/etc/security/cacerts",
	}

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
	logger := logging.GetLogger(ctx)
	logger.Warn("could not load any CA certificates for Termux")
	return pool
}
