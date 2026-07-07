package grim

import (
	"bytes"
	"context"
	"fmt"
	"image"
	_ "image/png"
	"strconv"
	"strings"
	"workspaced/internal/executil"
	"workspaced/pkg/driver"
	execdriver "workspaced/pkg/driver/exec"
	"workspaced/pkg/driver/screenshot"
	api "workspaced/pkg/driver/wm"
)

func init() {
	driver.Register[screenshot.Driver](&Factory{})
}

type Factory struct{}

func (f *Factory) ID() string   { return "screenshot_grim" }
func (f *Factory) Name() string { return "Grim (Wayland)" }

func (f *Factory) CheckCompatibility(ctx context.Context) error {
	if executil.GetEnv(ctx, "WAYLAND_DISPLAY") == "" {
		return fmt.Errorf("%w: WAYLAND_DISPLAY not set", driver.ErrIncompatible)
	}
	if !execdriver.IsBinaryAvailable(ctx, "grim") {
		return fmt.Errorf("%w: grim not found", driver.ErrIncompatible)
	}
	return nil
}

func (f *Factory) New(ctx context.Context) (screenshot.Driver, error) {
	return &Driver{}, nil
}

type Driver struct{}

func (d *Driver) SelectArea(ctx context.Context) (*api.Rect, error) {
	if !execdriver.IsBinaryAvailable(ctx, "slurp") {
		return nil, screenshot.ErrSelectionToolNotFound
	}
	out, err := execdriver.MustRun(ctx, "slurp").Output()
	if err != nil {
		return nil, err // likely canceled
	}
	raw := strings.TrimSpace(string(out))
	if raw == "" {
		return nil, screenshot.ErrEmptySelection
	}
	// slurp output: "x,y wxh"
	parts := strings.FieldsFunc(raw, func(r rune) bool {
		return r == ',' || r == ' ' || r == 'x'
	})
	if len(parts) != 4 {
		return nil, fmt.Errorf("invalid slurp output: %q", raw)
	}
	x, _ := strconv.Atoi(parts[0])
	y, _ := strconv.Atoi(parts[1])
	w, _ := strconv.Atoi(parts[2])
	h, _ := strconv.Atoi(parts[3])
	return &api.Rect{X: x, Y: y, Width: w, Height: h}, nil
}

func (d *Driver) Capture(ctx context.Context, rect *api.Rect) (image.Image, error) {
	args := []string{}
	if rect != nil {
		args = append(args, "-g", fmt.Sprintf("%d,%d %dx%d", rect.X, rect.Y, rect.Width, rect.Height))
	}

	args = append(args, "-") // Output to stdout

	cmd := execdriver.MustRun(ctx, "grim", args...)
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("grim failed: %w", err)
	}

	img, _, err := image.Decode(bytes.NewReader(out))
	if err != nil {
		return nil, fmt.Errorf("failed to decode grim output: %w", err)
	}

	return img, nil
}
