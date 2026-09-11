package feh

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

func (f *Factory) ID() string   { return "x11_feh" }
func (f *Factory) Name() string { return "X11 (feh)" }

func (f *Factory) CheckCompatibility(ctx context.Context) error {
	return execdriver.RequireEnvBinaries(ctx, "DISPLAY", "systemd-run", "feh")
}

func (f *Factory) New(ctx context.Context) (wallpaper.Driver, error) {
	return &Driver{}, nil
}

type Driver struct{}

func (d *Driver) SetStatic(ctx context.Context, path string) error {
	return wallpaper.SetStaticViaSystemdUnit(ctx, "feh", "--bg-fill", path)
}
