package kitty

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

func (f *Factory) ID() string   { return "terminal_kitty" }
func (f *Factory) Name() string { return "Kitty" }

func (f *Factory) CheckCompatibility(ctx context.Context) error {
	return execdriver.RequireBinary(ctx, "kitty")
}

func (f *Factory) New(ctx context.Context) (terminal.Driver, error) {
	return &Driver{}, nil
}

type Driver struct{}

func (d *Driver) Open(ctx context.Context, opts terminal.Options) error {
	args := []string{}
	if opts.Title != "" {
		args = append(args, "--title", opts.Title)
	}
	if opts.Command != "" {
		args = append(args, opts.Command)
		args = append(args, opts.Args...)
	}

	cmd := execdriver.MustRun(ctx, "kitty", args...)
	return cmd.Start()
}
