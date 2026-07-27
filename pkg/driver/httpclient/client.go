package httpclient

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"net"
	"net/http"
	"time"
)

// NewClient builds the shared HTTP client: force IPv4 dials, standard timeouts,
// optional root CAs, and WithProgress wrapping.
func NewClient(rootCAs *x509.CertPool, dialer *net.Dialer) *http.Client {
	if dialer == nil {
		dialer = &net.Dialer{
			Timeout:       30 * time.Second,
			KeepAlive:     30 * time.Second,
			FallbackDelay: 300 * time.Millisecond,
		}
	}
	inner := &http.Transport{
		DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
			if network == "tcp" {
				network = "tcp4"
			}
			return dialer.DialContext(ctx, network, addr)
		},
		TLSClientConfig: &tls.Config{
			RootCAs: rootCAs,
		},
		ForceAttemptHTTP2:     true,
		MaxIdleConns:          100,
		IdleConnTimeout:       90 * time.Second,
		TLSHandshakeTimeout:   10 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
	}
	return &http.Client{
		Transport: WithProgress(inner),
	}
}
