package termux

import (
	"context"
	"fmt"
	"image"
	"os"
	"strings"
	dapi "github.com/lucasew/workspaced/pkg/api"
	"github.com/lucasew/workspaced/pkg/driver"
	"github.com/lucasew/workspaced/pkg/driver/clipboard"
	execdriver "github.com/lucasew/workspaced/pkg/driver/exec"
)

func init() {
	driver.Register[clipboard.Driver](&Factory{})
}

type Factory struct{}

func (f *Factory) ID() string   { return "clipboard_termux" }
func (f *Factory) Name() string { return "Termux" }

func (f *Factory) CheckCompatibility(ctx context.Context) error {
	if os.Getenv("TERMUX_VERSION") == "" && !execdriver.IsBinaryAvailable(ctx, "termux-clipboard-set") {
		return fmt.Errorf("%w: termux not detected", driver.ErrIncompatible)
	}
	return nil
}

func (f *Factory) New(ctx context.Context) (clipboard.Driver, error) {
	return &Driver{}, nil
}

type Driver struct{}

func (d *Driver) WriteImage(ctx context.Context, img image.Image) error {
	return fmt.Errorf("%w: writing images to clipboard is not supported on Termux", dapi.ErrNotSupported)
}

func (d *Driver) WriteText(ctx context.Context, text string) error {
	if !execdriver.IsBinaryAvailable(ctx, "termux-clipboard-set") {
		return fmt.Errorf("%w: termux-clipboard-set (install termux-api)", dapi.ErrBinaryNotFound)
	}
	cmd := execdriver.MustRun(ctx, "termux-clipboard-set")
	cmd.Stdin = strings.NewReader(text)
	return cmd.Run()
}
