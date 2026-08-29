package must_test

import (
	"context"
	"errors"
	"testing"

	"github.com/lucasew/workspaced/internal/must"
)

func TestMustOK(t *testing.T) {
	t.Parallel()
	must.Must(func() error { return nil })
}

func TestMustPanics(t *testing.T) {
	t.Parallel()
	errBoom := errors.New("boom")
	defer func() {
		r := recover()
		if r != errBoom {
			t.Fatalf("recover = %v, want %v", r, errBoom)
		}
	}()
	must.Must(func() error { return errBoom })
	t.Fatal("expected panic")
}

func TestMustContext(t *testing.T) {
	t.Parallel()
	ctx := t.Context()
	var saw context.Context
	must.MustContext(ctx, func(c context.Context) error {
		saw = c
		return nil
	})
	if saw != ctx {
		t.Fatal("fn did not receive ctx")
	}

	errBoom := errors.New("boom")
	defer func() {
		if recover() != errBoom {
			t.Fatalf("recover mismatch")
		}
	}()
	must.MustContext(ctx, func(context.Context) error { return errBoom })
	t.Fatal("expected panic")
}

func TestValueAndValueContext(t *testing.T) {
	t.Parallel()
	if v := must.Value(func() (int, error) { return 7, nil }); v != 7 {
		t.Fatalf("Value = %d", v)
	}
	ctx := t.Context()
	if v := must.ValueContext(ctx, func(context.Context) (string, error) { return "ok", nil }); v != "ok" {
		t.Fatalf("ValueContext = %q", v)
	}
}

func TestErr(t *testing.T) {
	t.Parallel()
	must.Err(nil)
	errBoom := errors.New("boom")
	defer func() {
		if recover() != errBoom {
			t.Fatalf("recover mismatch")
		}
	}()
	must.Err(errBoom)
	t.Fatal("expected panic")
}
