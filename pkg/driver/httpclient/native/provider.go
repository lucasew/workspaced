package native

import (
	"context"
	"crypto/x509"
	"net/http"
	"os"
	"path/filepath"
	"sync"

	"github.com/lucasew/workspaced/pkg/driver"
	httpclientdriver "github.com/lucasew/workspaced/pkg/driver/httpclient"
	"github.com/lucasew/workspaced/pkg/logging"
)

func init() {
	driver.Register[httpclientdriver.Driver](&Factory{})
}

type Factory struct{}

func (f *Factory) ID() string   { return "httpclient_native" }
func (f *Factory) Name() string { return "Native HTTP Client" }

func (f *Factory) CheckCompatibility(ctx context.Context) error {
	// Always compatible
	return nil
}

func (f *Factory) New(ctx context.Context) (httpclientdriver.Driver, error) {
	return &Driver{
		rootCAs: loadSystemCerts(ctx),
	}, nil
}

type Driver struct {
	once    sync.Once
	client  *http.Client
	rootCAs *x509.CertPool
}

func (d *Driver) Client() *http.Client {
	d.once.Do(func() {
		// IPv4-first dial + WithProgress; see httpclient.NewClient.
		d.client = httpclientdriver.NewClient(d.rootCAs, nil)
	})
	return d.client
}

// loadSystemCerts attempts to load system CA certificates from multiple locations
func loadSystemCerts(ctx context.Context) *x509.CertPool {
	if pool, err := x509.SystemCertPool(); err == nil && pool != nil {
		return pool
	}

	// Fallback: create new pool and try common certificate locations
	pool := x509.NewCertPool()

	// Common certificate file locations (in priority order)
	certFiles := []string{
		"/etc/ssl/certs/ca-certificates.crt",
		"/etc/pki/tls/certs/ca-bundle.crt",
		"/etc/ssl/ca-bundle.pem",
		"/etc/pki/tls/cacert.pem",
		"/etc/pki/ca-trust/extracted/pem/tls-ca-bundle.pem",
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

	// Last resort: return the pool even if empty.
	// The TLS library may still work with built-in roots.
	logger := logging.GetLogger(ctx)
	logger.Warn("could not load system CA certificates from known locations")
	return pool
}
