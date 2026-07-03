package termux

import (
	"context"
	"fmt"
	"os"
	"workspaced/pkg/driver"
	execdriver "workspaced/pkg/driver/exec"
	"workspaced/pkg/driver/opener"
)

func init() {
	driver.Register[opener.Driver](&Factory{})
}

type Factory struct{}

func (f *Factory) ID() string   { return "opener_termux" }
func (f *Factory) Name() string { return "termux-open" }

func (f *Factory) CheckCompatibility(ctx context.Context) error {
	if os.Getenv("TERMUX_VERSION") == "" {
		return fmt.Errorf("%w: not running in Termux", driver.ErrIncompatible)
	}
	if !execdriver.IsBinaryAvailable(ctx, "termux-open") {
		return fmt.Errorf("%w: termux-open not found", driver.ErrIncompatible)
	}
	return nil
}

func (f *Factory) New(ctx context.Context) (opener.Driver, error) {
	return &Driver{}, nil
}

type Driver struct{}

func (d *Driver) Open(ctx context.Context, target string) error {
	return execdriver.MustRun(ctx, "termux-open", target).Start()
}
