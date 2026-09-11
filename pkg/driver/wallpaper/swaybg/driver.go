package swaybg

import (
	"context"

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
	return execdriver.RequireEnvBinaries(ctx, "WAYLAND_DISPLAY", "systemd-run", "swaybg")
}

func (f *Factory) New(ctx context.Context) (wallpaper.Driver, error) {
	return &Driver{}, nil
}

type Driver struct{}

func (d *Driver) SetStatic(ctx context.Context, path string) error {
	return wallpaper.SetStaticViaSystemdUnit(ctx, "swaybg", "-i", path)
}
