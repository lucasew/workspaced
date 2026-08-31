package wofi

import (
	"context"

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
	return execdriver.RequireEnvBinary(ctx, "WAYLAND_DISPLAY", "wofi")
}

type Driver struct{}

func (d *Driver) Choose(ctx context.Context, opts dialog.ChooseOptions) (*dialog.Item, error) {
	return dialog.ChooseViaCmd(ctx, opts, "wofi", false, "--dmenu", "-p", opts.Prompt)
}

func (d *Driver) RunApp(ctx context.Context) error {
	// wofi has no dedicated window switcher mode; reuse the app launcher.
	return execdriver.MustRun(ctx, "wofi", "--show", "drun").Run()
}

func (d *Driver) SwitchWindow(ctx context.Context) error {
	return d.RunApp(ctx)
}
