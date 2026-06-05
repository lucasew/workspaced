package provider

import (
	"context"
	"reflect"
	"workspaced/pkg/compat"
)

// Provider is the base interface for any data source in the system.
// Unlike drivers (which are chosen), providers are aggregated.
type Provider interface {
	// Name returns the unique identifier of the provider.
	// Examples: "node-npm", "python-pip", "go-mod".
	Name() string

	// Detect checks if this provider is applicable in the given context.
	// Return ErrNotApplicable when the provider should be skipped.
	Detect(ctx context.Context, dir string) error
}

// ErrIncompatible marks providers that do not apply to current context.
var ErrIncompatible = compat.ErrIncompatible

// ErrNotApplicable is kept as alias for readability in provider code.
var ErrNotApplicable = ErrIncompatible

// providers holds the registry of implementations.
// Since registration happens only during init(), we don't need mutexes for runtime access.
var providers = map[reflect.Type][]any{}

// Register adds a provider implementation to the global registry for a specific interface T.
func Register[T any](p T) {
	t := reflect.TypeFor[T]()
	providers[t] = append(providers[t], p)
}

// List returns all registered providers for the interface T.
func List[T any]() []T {
	t := reflect.TypeFor[T]()
	rawList := providers[t]

	result := make([]T, len(rawList))
	for i, raw := range rawList {
		result[i] = raw.(T)
	}
	return result
}
