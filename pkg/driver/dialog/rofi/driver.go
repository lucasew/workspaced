package rofi

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

func (f *ChooserFactory) ID() string                                      { return "rofi" }
func (f *ChooserFactory) Name() string                                    { return "Rofi" }
func (f *ChooserFactory) CheckCompatibility(ctx context.Context) error    { return checkRofi(ctx) }
func (f *ChooserFactory) New(ctx context.Context) (dialog.Chooser, error) { return &Driver{}, nil }

type FullDriverFactory struct{}

func (f *FullDriverFactory) ID() string                                     { return "rofi" }
func (f *FullDriverFactory) Name() string                                   { return "Rofi" }
func (f *FullDriverFactory) CheckCompatibility(ctx context.Context) error   { return checkRofi(ctx) }
func (f *FullDriverFactory) New(ctx context.Context) (dialog.Driver, error) { return &Driver{}, nil }

func checkRofi(ctx context.Context) error {
	return dialog.RequireDisplayBinary(ctx, "rofi")
}

type Driver struct{}

func (d *Driver) Choose(ctx context.Context, opts dialog.ChooseOptions) (*dialog.Item, error) {
	return dialog.ChooseViaCmd(ctx, opts, "rofi", true, "-dmenu", "-p", opts.Prompt, "-show-icons")
}

func (d *Driver) RunApp(ctx context.Context) error {
	return execdriver.MustRun(ctx, "rofi", "-show", "combi", "-combi-modi", "drun", "-show-icons").Run()
}

func (d *Driver) SwitchWindow(ctx context.Context) error {
	return execdriver.MustRun(ctx, "rofi", "-show", "combi", "-combi-modi", "window", "-show-icons").Run()
}
