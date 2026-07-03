package xdg

import (
	"context"
	"fmt"
	"workspaced/pkg/driver"
	execdriver "workspaced/pkg/driver/exec"
	"workspaced/pkg/driver/opener"
	"workspaced/pkg/executil"
)

func init() {
	driver.Register[opener.Driver](&Factory{})
}

type Factory struct{}

func (f *Factory) ID() string   { return "opener_xdg" }
func (f *Factory) Name() string { return "xdg-open" }

func (f *Factory) CheckCompatibility(ctx context.Context) error {
	if executil.GetEnv(ctx, "DISPLAY") == "" && executil.GetEnv(ctx, "WAYLAND_DISPLAY") == "" {
		return fmt.Errorf("%w: neither DISPLAY nor WAYLAND_DISPLAY set", driver.ErrIncompatible)
	}
	if !execdriver.IsBinaryAvailable(ctx, "xdg-open") {
		return fmt.Errorf("%w: xdg-open not found", driver.ErrIncompatible)
	}
	return nil
}

func (f *Factory) New(ctx context.Context) (opener.Driver, error) {
	return &Driver{}, nil
}

type Driver struct{}

func (d *Driver) Open(ctx context.Context, target string) error {
	return execdriver.MustRun(ctx, "xdg-open", target).Start()
}
