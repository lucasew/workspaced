package alacritty

import (
	"context"
	"fmt"
	"workspaced/pkg/driver"
	execdriver "workspaced/pkg/driver/exec"
	"workspaced/pkg/driver/terminal"
)

func init() {
	driver.Register[terminal.Driver](&Factory{})
}

type Factory struct{}

func (p *Factory) ID() string   { return "terminal_alacritty" }
func (p *Factory) Name() string { return "Alacritty" }

func (p *Factory) CheckCompatibility(ctx context.Context) error {
	if !execdriver.IsBinaryAvailable(ctx, "alacritty") {
		return fmt.Errorf("%w: alacritty not found", driver.ErrIncompatible)
	}
	return nil
}

func (p *Factory) New(ctx context.Context) (terminal.Driver, error) {
	return &Driver{}, nil
}

type Driver struct{}

func (d *Driver) Open(ctx context.Context, opts terminal.Options) error {
	args := []string{}
	if opts.Title != "" {
		args = append(args, "-T", opts.Title)
	}
	if opts.Command != "" {
		args = append(args, "-e", opts.Command)
		args = append(args, opts.Args...)
	}

	cmd := execdriver.MustRun(ctx, "alacritty", args...)
	return cmd.Start()
}
