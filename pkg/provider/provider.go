package provider

import "context"

// Provider is the base interface for any data source in the system.
// Unlike drivers (which are chosen), providers are aggregated.
type Provider interface {
	// Name returns the unique identifier of the provider.
	// Examples: "node-npm", "python-pip", "go-mod".
	Name() string

	// Detect checks if this provider is applicable in the given context.
	// Typically checks for the existence of specific files (e.g. package.json).
	Detect(ctx context.Context, dir string) (bool, error)
}
