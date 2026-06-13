package termux

import (
	"context"
	"fmt"
	"os"
	"workspaced/pkg/api"
	"workspaced/pkg/driver"
	execdriver "workspaced/pkg/driver/exec"
	"workspaced/pkg/driver/power"
)

func init() {
	driver.Register[power.Driver](&Factory{})
}

type Factory struct{}

func (p *Factory) ID() string   { return "power_termux" }
func (p *Factory) Name() string { return "Termux" }

func (p *Factory) CheckCompatibility(ctx context.Context) error {
	if os.Getenv("TERMUX_VERSION") == "" {
		return fmt.Errorf("%w: not running in Termux", driver.ErrIncompatible)
	}
	return nil
}

func (p *Factory) New(ctx context.Context) (power.Driver, error) {
	return &Driver{}, nil
}

type Driver struct{}

func (d *Driver) Lock(ctx context.Context) error {
	return fmt.Errorf("%w: screen lock not possible in Termux", api.ErrNotSupported)
}

func (d *Driver) Logout(ctx context.Context) error {
	return fmt.Errorf("%w: logout not possible in Termux", api.ErrNotSupported)
}

func (d *Driver) Suspend(ctx context.Context) error {
	return fmt.Errorf("%w: suspend not possible in Termux", api.ErrNotSupported)
}

func (d *Driver) Hibernate(ctx context.Context) error {
	return fmt.Errorf("%w: hibernate not possible in Termux", api.ErrNotSupported)
}

func (d *Driver) Reboot(ctx context.Context) error {
	// If rooted, might work.
	return execdriver.MustRun(ctx, "reboot").Run()
}

func (d *Driver) Shutdown(ctx context.Context) error {
	// If rooted, might work.
	return execdriver.MustRun(ctx, "shutdown", "-h", "now").Run()
}
