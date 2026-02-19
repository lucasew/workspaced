package native

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
)

func init() {
	driver.Register[httpclientdriver.Driver](&Provider{})
}

type Provider struct{}

func (p *Provider) ID() string         { return "httpclient_native" }
func (p *Provider) Name() string       { return "Native HTTP Client" }
func (p *Provider) DefaultWeight() int { return driver.DefaultWeight }

func (p *Provider) CheckCompatibility(ctx context.Context) error {
	// Always compatible
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
		// Create custom dialer that prefers IPv4 and has proper timeouts
		dialer := &net.Dialer{
			Timeout:   30 * time.Second,
			KeepAlive: 30 * time.Second,
			// Prefer IPv4 over IPv6 to avoid Termux DNS issues
			FallbackDelay: 300 * time.Millisecond,
		}

		d.client = &http.Client{
			Transport: &http.Transport{
				DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
					// Try IPv4 first
					if network == "tcp" {
						network = "tcp4"
					}
					return dialer.DialContext(ctx, network, addr)
				},
				TLSClientConfig: &tls.Config{
					RootCAs: loadSystemCerts(),
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

// loadSystemCerts attempts to load system CA certificates from multiple locations
func loadSystemCerts() *x509.CertPool {
	// Try to load system cert pool first (works on most platforms)
	if pool, err := x509.SystemCertPool(); err == nil && pool != nil {
		return pool
	}

	// Fallback: create new pool and try common certificate locations
	pool := x509.NewCertPool()

	// Common certificate file locations (in priority order)
	certFiles := []string{
		// Standard Linux locations
		"/etc/ssl/certs/ca-certificates.crt",
		"/etc/pki/tls/certs/ca-bundle.crt",
		"/etc/ssl/ca-bundle.pem",
		"/etc/pki/tls/cacert.pem",
		"/etc/pki/ca-trust/extracted/pem/tls-ca-bundle.pem",
		// Termux locations
		"/data/data/com.termux/files/usr/etc/tls/cert.pem",
		"/system/etc/security/cacerts",
	}

	// Also check environment variables
	if certFile := os.Getenv("SSL_CERT_FILE"); certFile != "" {
		certFiles = append([]string{certFile}, certFiles...)
	}
	if certDir := os.Getenv("SSL_CERT_DIR"); certDir != "" {
		certFiles = append([]string{filepath.Join(certDir, "ca-certificates.crt")}, certFiles...)
	}

	// Try to load from each location
	for _, certFile := range certFiles {
		if certs, err := os.ReadFile(certFile); err == nil {
			if pool.AppendCertsFromPEM(certs) {
				return pool
			}
		}
	}

	// Try loading from directory (for Android/Termux)
	certDirs := []string{
		"/system/etc/security/cacerts",
		"/data/data/com.termux/files/usr/etc/tls/certs",
	}

	for _, certDir := range certDirs {
		entries, err := os.ReadDir(certDir)
		if err != nil {
			continue
		}
		addedAny := false
		for _, entry := range entries {
			if entry.IsDir() {
				continue
			}
			certPath := filepath.Join(certDir, entry.Name())
			if certs, err := os.ReadFile(certPath); err == nil {
				if pool.AppendCertsFromPEM(certs) {
					addedAny = true
				}
			}
		}
		// If we loaded any certs from this directory, return
		if addedAny {
			return pool
		}
	}

	// Last resort: return the pool even if empty
	// The TLS library may still work with built-in roots
	fmt.Fprintf(os.Stderr, "Warning: Could not load system CA certificates from any known location\n")
	return pool
}
