package wofi

import (
	"context"
	"fmt"
	"strings"
	"github.com/lucasew/workspaced/internal/executil"
	"github.com/lucasew/workspaced/pkg/driver"
	"github.com/lucasew/workspaced/pkg/driver/dialog"
	execdriver "github.com/lucasew/workspaced/pkg/driver/exec"
)

func init() {
	driver.Register[dialog.Chooser](&ChooserFactory{})
	driver.Register[dialog.Driver](&FullDriverFactory{})
}

type ChooserFactory struct{}

func (f *ChooserFactory) ID() string                                      { return "wofi" }
func (f *ChooserFactory) Name() string                                    { return "Wofi" }
func (f *ChooserFactory) CheckCompatibility(ctx context.Context) error    { return checkWofi(ctx) }
func (f *ChooserFactory) New(ctx context.Context) (dialog.Chooser, error) { return &Driver{}, nil }

type FullDriverFactory struct{}

func (f *FullDriverFactory) ID() string                                     { return "wofi" }
func (f *FullDriverFactory) Name() string                                   { return "Wofi" }
func (f *FullDriverFactory) CheckCompatibility(ctx context.Context) error   { return checkWofi(ctx) }
func (f *FullDriverFactory) New(ctx context.Context) (dialog.Driver, error) { return &Driver{}, nil }

func checkWofi(ctx context.Context) error {
	if executil.GetEnv(ctx, "WAYLAND_DISPLAY") == "" {
		return fmt.Errorf("%w: WAYLAND_DISPLAY not set", driver.ErrIncompatible)
	}
	if !execdriver.IsBinaryAvailable(ctx, "wofi") {
		return fmt.Errorf("%w: wofi not found", driver.ErrIncompatible)
	}
	return nil
}

type Driver struct{}

func (d *Driver) Choose(ctx context.Context, opts dialog.ChooseOptions) (*dialog.Item, error) {
	var input strings.Builder
	for _, item := range opts.Items {
		label := item.Label
		if label == "" {
			label = item.Value
		}
		input.WriteString(label)
		input.WriteString("\n")
	}

	args := []string{"--dmenu", "-p", opts.Prompt}

	cmd := execdriver.MustRun(ctx, "wofi", args...)
	cmd.Stdin = strings.NewReader(input.String())

	out, err := cmd.Output()
	if err != nil {
		return nil, err
	}

	selected := strings.TrimSpace(string(out))
	if selected == "" {
		return nil, nil
	}

	for _, item := range opts.Items {
		label := item.Label
		if label == "" {
			label = item.Value
		}
		if selected == label {
			return &item, nil
		}
	}

	return &dialog.Item{Label: selected, Value: selected}, nil
}

func (d *Driver) RunApp(ctx context.Context) error {
	return execdriver.MustRun(ctx, "wofi", "--show", "drun").Run()
}

func (d *Driver) SwitchWindow(ctx context.Context) error {
	return execdriver.MustRun(ctx, "wofi", "--show", "drun").Run()
}
