package httpclient

import (
	"net/http"

	"workspaced/pkg/logging"
)

// Driver provides an HTTP client with platform-specific certificate handling.
type Driver interface {
	// Client returns a configured HTTP client with proper certificate handling
	Client() *http.Client
}

// WithLogging wraps a Driver so that every HTTP request logs the URL at Debug level.
func WithLogging(d Driver) Driver {
	return &loggingDriver{inner: d}
}

type loggingDriver struct {
	inner Driver
}

func (d *loggingDriver) Client() *http.Client {
	c := d.inner.Client()
	base := c.Transport
	if base == nil {
		base = http.DefaultTransport
	}
	clone := *c
	clone.Transport = &loggingTransport{base: base}
	return &clone
}

type loggingTransport struct {
	base http.RoundTripper
}

func (t *loggingTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	logging.GetLogger(req.Context()).Debug("http request", "url", req.URL.String())
	return t.base.RoundTrip(req)
}
