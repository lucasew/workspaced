package foot

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

func (f *Factory) ID() string   { return "terminal_foot" }
func (f *Factory) Name() string { return "Foot" }

func (f *Factory) CheckCompatibility(ctx context.Context) error {
	if err := driver.RequireEnv(ctx, "WAYLAND_DISPLAY"); err != nil {
		return err
	}
	return execdriver.RequireBinary(ctx, "foot")
}

func (f *Factory) New(ctx context.Context) (terminal.Driver, error) {
	return &Driver{}, nil
}

type Driver struct{}

func (d *Driver) Open(ctx context.Context, opts terminal.Options) error {
	args := []string{}
	if opts.Title != "" {
		args = append(args, "-T", opts.Title)
	}
	if opts.Command != "" {
		args = append(args, opts.Command)
		args = append(args, opts.Args...)
	}

	cmd := execdriver.MustRun(ctx, "foot", args...)
	return cmd.Start()
}
