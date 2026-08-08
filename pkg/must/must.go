// Package must provides panic-on-error helpers for infallible call sites
// (package init, tests, demos, one-shot setup).
//
// Do not use on normal command paths that should propagate errors to the CLI.
// Prefer returning error there; reserve must for cases where failure means the
// process cannot continue meaningfully (same spirit as template.Must).
package must

import "context"

// Must runs fn and panics if it returns a non-nil error.
func Must(fn func() error) {
	if err := fn(); err != nil {
		panic(err)
	}
}

// MustContext runs fn(ctx) and panics if it returns a non-nil error.
func MustContext(ctx context.Context, fn func(context.Context) error) {
	if err := fn(ctx); err != nil {
		panic(err)
	}
}

// Value runs fn and panics if it returns a non-nil error; otherwise returns v.
func Value[T any](fn func() (T, error)) T {
	v, err := fn()
	if err != nil {
		panic(err)
	}
	return v
}

// ValueContext runs fn(ctx) and panics if it returns a non-nil error; otherwise returns v.
func ValueContext[T any](ctx context.Context, fn func(context.Context) (T, error)) T {
	v, err := fn(ctx)
	if err != nil {
		panic(err)
	}
	return v
}

// Err panics if err is non-nil. Handy next to calls that already unpack values:
//
//	v, err := f()
//	must.Err(err)
func Err(err error) {
	if err != nil {
		panic(err)
	}
}
