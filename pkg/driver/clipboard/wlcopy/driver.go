package wlcopy

import (
	"context"
	"fmt"
	"image"
	"image/png"
	"io"
	"strings"
	"github.com/lucasew/workspaced/internal/executil"
	dapi "github.com/lucasew/workspaced/pkg/api"
	"github.com/lucasew/workspaced/pkg/driver"
	"github.com/lucasew/workspaced/pkg/driver/clipboard"
	execdriver "github.com/lucasew/workspaced/pkg/driver/exec"
	"github.com/lucasew/workspaced/pkg/logging"
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
	if !execdriver.IsBinaryAvailable(ctx, "wl-copy") {
		return fmt.Errorf("%w: wl-copy", dapi.ErrBinaryNotFound)
	}
	pr, pw := io.Pipe()
	go func() {
		if err := png.Encode(pw, img); err != nil {
			logging.ReportError(ctx, err)
		}
		logging.Close(ctx, pw)
	}()

	cmd := execdriver.MustRun(ctx, "wl-copy", "-t", "image/png")
	cmd.Stdin = pr
	return cmd.Run()
}

func (d *Driver) WriteText(ctx context.Context, text string) error {
	if !execdriver.IsBinaryAvailable(ctx, "wl-copy") {
		return fmt.Errorf("%w: wl-copy", dapi.ErrBinaryNotFound)
	}
	cmd := execdriver.MustRun(ctx, "wl-copy")
	cmd.Stdin = strings.NewReader(text)
	return cmd.Run()
}
