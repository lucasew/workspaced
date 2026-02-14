package httpclient

import (
	"net/http"
)

// Driver provides an HTTP client with platform-specific certificate handling.
type Driver interface {
	// Client returns a configured HTTP client with proper certificate handling
	Client() *http.Client
}
