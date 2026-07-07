package foot

import (
	"context"
	"fmt"
	"workspaced/internal/executil"
	"workspaced/pkg/driver"
	execdriver "workspaced/pkg/driver/exec"
	"workspaced/pkg/driver/terminal"
)

func init() {
	driver.Register[terminal.Driver](&Factory{})
}

type Factory struct{}

func (f *Factory) ID() string   { return "terminal_foot" }
func (f *Factory) Name() string { return "Foot" }

func (f *Factory) CheckCompatibility(ctx context.Context) error {
	if executil.GetEnv(ctx, "WAYLAND_DISPLAY") == "" {
		return fmt.Errorf("%w: foot requires WAYLAND_DISPLAY", driver.ErrIncompatible)
	}
	if !execdriver.IsBinaryAvailable(ctx, "foot") {
		return fmt.Errorf("%w: foot not found", driver.ErrIncompatible)
	}
	return nil
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
