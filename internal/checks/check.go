// Package checks runs aggregated linters and formatters declared in CUE
// (workspaced.lint / workspaced.formatter). Unlike drivers (one impl by weight),
// every tool whose detect firewall matches is run.
//
// See docs/specs/checks-cue-review.md.
package checks

import (
	"context"
	"errors"
	"github.com/lucasew/workspaced/internal/compat"
	"reflect"
)

// Check is the base interface for discoverable, directory-applicable actions
// (linters, formatters, etc.). All that pass Detect() are aggregated.
type Check interface {
	// Name returns the unique identifier of the check.
	// Examples: "golangci-lint", "prettier".
	Name() string

	// Detect checks if this check is applicable in the given context (e.g. does
	// the project contain files this tool understands?).
	// Return ErrNotApplicable when the check should be skipped for this dir.
	Detect(ctx context.Context, dir string) error
}

// ErrIncompatible marks checks that do not apply to current context.
var ErrIncompatible = compat.ErrIncompatible

// ErrNotApplicable is kept as alias for readability in check code.
var ErrNotApplicable = ErrIncompatible

// ErrToolNotAvailable indicates a required tool binary is not available.
var ErrToolNotAvailable = errors.New("required tool not available")

// registry holds implementations keyed by interface type.
// Since registration happens only during init(), we don't need mutexes for runtime access.
var registry = map[reflect.Type][]any{}

// Register adds an implementation to the global registry for a specific interface T.
func Register[T any](impl T) {
	t := reflect.TypeFor[T]()
	registry[t] = append(registry[t], impl)
}

// List returns all registered implementations for the interface T.
func List[T any]() []T {
	t := reflect.TypeFor[T]()
	rawList := registry[t]

	result := make([]T, len(rawList))
	for i, raw := range rawList {
		result[i] = raw.(T)
	}
	return result
}
