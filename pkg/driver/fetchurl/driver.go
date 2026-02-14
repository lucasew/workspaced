package fetchurl

import (
	"context"
	"io"
)

// FetchOptions configures a download operation
type FetchOptions struct {
	// URLs to try downloading from (in order)
	URLs []string
	// Hash algorithm (e.g., "sha256", "sha512")
	Algo string
	// Expected hash value
	Hash string
	// Output destination
	Out io.Writer
}

// Driver provides hash-verified downloads
type Driver interface {
	// Fetch downloads a file with hash verification
	// Tries URLs in order until one succeeds
	Fetch(ctx context.Context, opts FetchOptions) error
}
