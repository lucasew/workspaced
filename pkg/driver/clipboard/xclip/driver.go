package xclip

import (
	"context"
	"fmt"
	dapi "github.com/lucasew/workspaced/pkg/api"
	"github.com/lucasew/workspaced/pkg/driver"
	"github.com/lucasew/workspaced/pkg/driver/clipboard"
	execdriver "github.com/lucasew/workspaced/pkg/driver/exec"
	"github.com/lucasew/workspaced/pkg/logging"
	"image"
	"image/png"
	"io"
	"strings"
)

func init() {
	driver.Register[clipboard.Driver](&Factory{})
}

type Factory struct{}

func (f *Factory) ID() string   { return "clipboard_xclip" }
func (f *Factory) Name() string { return "X11 (xclip)" }

func (f *Factory) CheckCompatibility(ctx context.Context) error {
	if !execdriver.IsBinaryAvailable(ctx, "xclip") {
		return fmt.Errorf("%w: xclip", driver.ErrIncompatible)
	}
	// Fallback driver, usually always valid if binary exists
	return nil
}

func (f *Factory) New(ctx context.Context) (clipboard.Driver, error) {
	return &Driver{}, nil
}

type Driver struct{}

func (d *Driver) WriteImage(ctx context.Context, img image.Image) error {
	if !execdriver.IsBinaryAvailable(ctx, "xclip") {
		return fmt.Errorf("%w: xclip", dapi.ErrBinaryNotFound)
	}
	pr, pw := io.Pipe()
	go func() {
		if err := png.Encode(pw, img); err != nil {
			logging.ReportError(ctx, err)
		}
		logging.Close(ctx, pw)
	}()

	cmd := execdriver.MustRun(ctx, "xclip", "-selection", "clipboard", "-t", "image/png")
	cmd.Stdin = pr
	return cmd.Run()
}

func (d *Driver) WriteText(ctx context.Context, text string) error {
	if !execdriver.IsBinaryAvailable(ctx, "xclip") {
		return fmt.Errorf("%w: xclip", dapi.ErrBinaryNotFound)
	}
	cmd := execdriver.MustRun(ctx, "xclip", "-selection", "clipboard")
	cmd.Stdin = strings.NewReader(text)
	return cmd.Run()
}
