package wlcopy

import (
	"context"
	"fmt"
	"image"

	"github.com/lucasew/workspaced/internal/executil"
	"github.com/lucasew/workspaced/pkg/driver"
	"github.com/lucasew/workspaced/pkg/driver/clipboard"
	execdriver "github.com/lucasew/workspaced/pkg/driver/exec"
)

func init() {
	driver.Register[clipboard.Driver](&Factory{})
}

type Factory struct{}

func (f *Factory) ID() string   { return "clipboard_wlcopy" }
func (f *Factory) Name() string { return "Wayland (wl-copy)" }

func (f *Factory) CheckCompatibility(ctx context.Context) error {
	if executil.GetEnv(ctx, "WAYLAND_DISPLAY") == "" {
		return fmt.Errorf("%w: WAYLAND_DISPLAY not set", driver.ErrIncompatible)
	}
	if !execdriver.IsBinaryAvailable(ctx, "wl-copy") {
		return fmt.Errorf("%w: wl-copy not found", driver.ErrIncompatible)
	}
	return nil
}

func (f *Factory) New(ctx context.Context) (clipboard.Driver, error) {
	return &Driver{}, nil
}

type Driver struct{}

func (d *Driver) WriteImage(ctx context.Context, img image.Image) error {
	return clipboard.WriteImageViaCmd(ctx, img, "wl-copy", "-t", "image/png")
}

func (d *Driver) WriteText(ctx context.Context, text string) error {
	return clipboard.WriteTextViaCmd(ctx, text, "wl-copy")
}
