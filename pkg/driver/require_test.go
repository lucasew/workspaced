package driver_test

import (
	"context"
	"errors"
	"testing"

	"workspaced/pkg/driver"
	"workspaced/pkg/executil"
)

func TestRequireEnv(t *testing.T) {
	ctx := executil.WithEnv(context.Background(), []string{"FOO=1"})
	if err := driver.RequireEnv(ctx, "FOO"); err != nil {
		t.Fatalf("present: %v", err)
	}
	err := driver.RequireEnv(ctx, "MISSING")
	if !errors.Is(err, driver.ErrIncompatible) {
		t.Fatalf("missing: %v", err)
	}
}

func TestRequireAnyEnv(t *testing.T) {
	ctx := executil.WithEnv(context.Background(), []string{"WAYLAND_DISPLAY=wayland-0"})
	if err := driver.RequireAnyEnv(ctx, "DISPLAY", "WAYLAND_DISPLAY"); err != nil {
		t.Fatalf("any present: %v", err)
	}
	err := driver.RequireAnyEnv(ctx, "DISPLAY", "OTHER")
	if !errors.Is(err, driver.ErrIncompatible) {
		t.Fatalf("none: %v", err)
	}
}

func TestRequireTermux(t *testing.T) {
	t.Setenv("TERMUX_VERSION", "")
	if err := driver.RequireTermux(); !errors.Is(err, driver.ErrIncompatible) {
		t.Fatalf("unset: %v", err)
	}
	t.Setenv("TERMUX_VERSION", "0.118")
	if err := driver.RequireTermux(); err != nil {
		t.Fatalf("set: %v", err)
	}
	if !driver.IsTermux() {
		t.Fatal("IsTermux false")
	}
}
