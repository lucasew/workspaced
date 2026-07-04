package driver

import "context"

// With loads the active driver for T and runs fn.
// It collapses the Get-then-delegate boilerplate used by thin facades.
func With[T any](ctx context.Context, fn func(T) error) error {
	d, err := Get[T](ctx)
	if err != nil {
		return err
	}
	return fn(d)
}

// WithResult is With for functions that return a value.
func WithResult[T, R any](ctx context.Context, fn func(T) (R, error)) (R, error) {
	var zero R
	d, err := Get[T](ctx)
	if err != nil {
		return zero, err
	}
	return fn(d)
}
