package fetchurl

import (
	"context"
	"errors"
	"io"
)

var (
	ErrNoURLs         = errors.New("no URLs provided")
	ErrNoOutputWriter = errors.New("no output writer provided")
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
	// Size is the expected total size in bytes (if known). Used to drive
	// determinate progress reporting for the taskgroup UI when a Group is
	// present in ctx. 0 or negative means unknown (indeterminate).
	Size int64
}

// Driver provides hash-verified downloads
type Driver interface {
	// Fetch downloads a file with hash verification
	// Tries URLs in order until one succeeds
	Fetch(ctx context.Context, opts FetchOptions) error
}
