package xclip

import (
	"context"
	"github.com/lucasew/workspaced/pkg/driver"
	"github.com/lucasew/workspaced/pkg/driver/clipboard"
	execdriver "github.com/lucasew/workspaced/pkg/driver/exec"
	"image"
)

func init() {
	driver.Register[clipboard.Driver](&Factory{})
}

type Factory struct{}

func (f *Factory) ID() string   { return "clipboard_xclip" }
func (f *Factory) Name() string { return "X11 (xclip)" }

func (f *Factory) CheckCompatibility(ctx context.Context) error {
	// Fallback driver, usually always valid if binary exists
	return execdriver.RequireBinary(ctx, "xclip")
}

func (f *Factory) New(ctx context.Context) (clipboard.Driver, error) {
	return &Driver{}, nil
}

type Driver struct{}

func (d *Driver) WriteImage(ctx context.Context, img image.Image) error {
	return clipboard.WriteImageViaCmd(ctx, img, "xclip", "-selection", "clipboard", "-t", "image/png")
}

func (d *Driver) WriteText(ctx context.Context, text string) error {
	return clipboard.WriteTextViaCmd(ctx, text, "xclip", "-selection", "clipboard")
}
