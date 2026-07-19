package maim

import (
	"bytes"
	"context"
	"fmt"
	"image"
	_ "image/png"
	"strings"

	"github.com/lucasew/workspaced/internal/executil"
	"github.com/lucasew/workspaced/pkg/driver"
	execdriver "github.com/lucasew/workspaced/pkg/driver/exec"
	"github.com/lucasew/workspaced/pkg/driver/screenshot"
	api "github.com/lucasew/workspaced/pkg/driver/wm"
)

func init() {
	driver.Register[screenshot.Driver](&Factory{})
}

type Factory struct{}

func (f *Factory) ID() string   { return "screenshot_maim" }
func (f *Factory) Name() string { return "Maim (X11)" }

func (f *Factory) CheckCompatibility(ctx context.Context) error {
	if executil.GetEnv(ctx, "DISPLAY") == "" {
		return fmt.Errorf("%w: DISPLAY not set", driver.ErrIncompatible)
	}
	if !execdriver.IsBinaryAvailable(ctx, "maim") {
		return fmt.Errorf("%w: maim not found", driver.ErrIncompatible)
	}
	return nil
}

func (f *Factory) New(ctx context.Context) (screenshot.Driver, error) {
	return &Driver{}, nil
}

type Driver struct{}

func (d *Driver) SelectArea(ctx context.Context) (*api.Rect, error) {
	// maim uses slop for selection.
	// maim -g $(slop) ... is common.
	// We can run slop directly to get the geometry.
	if !execdriver.IsBinaryAvailable(ctx, "slop") {
		return nil, screenshot.ErrSelectionToolNotFound
	}
	out, err := execdriver.MustRun(ctx, "slop", "-f", "%x %y %w %h").Output()
	if err != nil {
		return nil, err // likely canceled
	}
	raw := strings.TrimSpace(string(out))
	parts := strings.Fields(raw)
	rect, err := screenshot.ParseRectParts(parts)
	if err != nil {
		return nil, fmt.Errorf("invalid slop output %q: %w", raw, err)
	}
	return rect, nil
}

func (d *Driver) Capture(ctx context.Context, rect *api.Rect) (image.Image, error) {
	args := []string{}
	if rect != nil {
		// maim geometry format: WxH+X+Y
		args = append(args, "-g", fmt.Sprintf("%dx%d+%d+%d", rect.Width, rect.Height, rect.X, rect.Y))
	}

	cmd := execdriver.MustRun(ctx, "maim", args...)
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("maim failed: %w", err)
	}

	img, _, err := image.Decode(bytes.NewReader(out))
	if err != nil {
		return nil, fmt.Errorf("decode maim output: %w", err)
	}

	return img, nil
}
