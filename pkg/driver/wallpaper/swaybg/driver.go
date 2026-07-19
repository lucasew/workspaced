package swaybg

import (
	"context"
	"fmt"
	"github.com/lucasew/workspaced/internal/executil"
	"github.com/lucasew/workspaced/pkg/driver"
	execdriver "github.com/lucasew/workspaced/pkg/driver/exec"
	"github.com/lucasew/workspaced/pkg/driver/wallpaper"
)

func init() {
	driver.Register[wallpaper.Driver](&Factory{})
}

type Factory struct{}

func (f *Factory) ID() string   { return "wayland_swaybg" }
func (f *Factory) Name() string { return "Wayland (swaybg)" }

func (f *Factory) CheckCompatibility(ctx context.Context) error {
	if executil.GetEnv(ctx, "WAYLAND_DISPLAY") == "" {
		return fmt.Errorf("%w: WAYLAND_DISPLAY not set", driver.ErrIncompatible)
	}
	if _, err := execdriver.Which(ctx, "systemd-run"); err != nil {
		return fmt.Errorf("%w: systemd-run not found", driver.ErrIncompatible)
	}
	if _, err := execdriver.Which(ctx, "swaybg"); err != nil {
		return fmt.Errorf("%w: swaybg not found", driver.ErrIncompatible)
	}
	return nil
}

func (f *Factory) New(ctx context.Context) (wallpaper.Driver, error) {
	return &Driver{}, nil
}

type Driver struct{}

func (d *Driver) SetStatic(ctx context.Context, path string) error {
	swaybg, err := execdriver.Which(ctx, "swaybg")
	if err != nil {
		return err
	}

	if err = execdriver.MustRun(ctx, "systemd-run", "--user", "-u", "wallpaper-change", "--collect", swaybg, "-i", path).Run(); err != nil {
		return fmt.Errorf("can't run swaybg in systemd unit: %w", err)
	}
	return nil
}
