package alacritty

import (
	"context"
	"github.com/lucasew/workspaced/pkg/driver"
	execdriver "github.com/lucasew/workspaced/pkg/driver/exec"
	"github.com/lucasew/workspaced/pkg/driver/terminal"
)

func init() {
	driver.Register[terminal.Driver](&Factory{})
}

type Factory struct{}

func (f *Factory) ID() string   { return "terminal_alacritty" }
func (f *Factory) Name() string { return "Alacritty" }

func (f *Factory) CheckCompatibility(ctx context.Context) error {
	return execdriver.RequireBinary(ctx, "alacritty")
}

func (f *Factory) New(ctx context.Context) (terminal.Driver, error) {
	return &Driver{}, nil
}

type Driver struct{}

func (d *Driver) Open(ctx context.Context, opts terminal.Options) error {
	cmd := execdriver.MustRun(ctx, "alacritty", terminal.BuildOpenArgs(opts, "-T", true)...)
	return cmd.Start()
}
