package systemd

import (
	"context"
	"github.com/lucasew/workspaced/pkg/driver"
	execdriver "github.com/lucasew/workspaced/pkg/driver/exec"
	"github.com/lucasew/workspaced/pkg/driver/power"
)

func init() {
	driver.Register[power.Driver](&Factory{})
}

type Factory struct{}

func (f *Factory) ID() string   { return "power_systemd" }
func (f *Factory) Name() string { return "Systemd" }

func (f *Factory) CheckCompatibility(ctx context.Context) error {
	return execdriver.RequireBinaries(ctx, "loginctl", "systemctl")
}

func (f *Factory) New(ctx context.Context) (power.Driver, error) {
	return &Driver{}, nil
}

type Driver struct{}

func (d *Driver) Lock(ctx context.Context) error {
	return execdriver.MustRun(ctx, "loginctl", "lock-session").Run()
}

func (d *Driver) Logout(ctx context.Context) error {
	return execdriver.MustRun(ctx, "loginctl", "terminate-session", "self").Run()
}

func (d *Driver) Suspend(ctx context.Context) error {
	return execdriver.MustRun(ctx, "systemctl", "suspend").Run()
}

func (d *Driver) Hibernate(ctx context.Context) error {
	return execdriver.MustRun(ctx, "systemctl", "hibernate").Run()
}

func (d *Driver) Reboot(ctx context.Context) error {
	return execdriver.MustRun(ctx, "systemctl", "reboot").Run()
}

func (d *Driver) Shutdown(ctx context.Context) error {
	return execdriver.MustRun(ctx, "systemctl", "poweroff").Run()
}
