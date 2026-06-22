package x11

import (
	"context"
	"fmt"
	"strings"
	"workspaced/pkg/api"
	"workspaced/pkg/driver"
	execdriver "workspaced/pkg/driver/exec"
	"workspaced/pkg/driver/screen"
	envdriver "workspaced/pkg/driver/env"
	"workspaced/pkg/executil"
)

func init() {
	driver.Register[screen.Driver](&Factory{})
}

type Factory struct{}

func (p *Factory) ID() string   { return "screen_x11" }
func (p *Factory) Name() string { return "X11" }

func (p *Factory) CheckCompatibility(ctx context.Context) error {
	display := executil.GetEnv(ctx, "DISPLAY")

	if display == "" {
		return fmt.Errorf("%w: DISPLAY not set", driver.ErrIncompatible)
	}
	if _, err := execdriver.Which(ctx, "xset"); err != nil {
		return fmt.Errorf("%w: xset not found", driver.ErrIncompatible)
	}
	if _, err := execdriver.Which(ctx, "xrandr"); err != nil {
		return fmt.Errorf("%w: xrandr not found", driver.ErrIncompatible)
	}
	return nil
}

func (p *Factory) New(ctx context.Context) (screen.Driver, error) {
	return &Driver{}, nil
}

type Driver struct{}

func (d *Driver) SetDPMS(ctx context.Context, on bool) error {
	xsetArg := "off"
	if on {
		xsetArg = "on"
	}
	return execdriver.MustRun(ctx, "xset", "dpms", "force", xsetArg).Run()
}

func (d *Driver) IsDPMSOn(ctx context.Context) (bool, error) {
	out, err := execdriver.MustRun(ctx, "xset", "q").Output()
	if err != nil {
		return false, err
	}
	return strings.Contains(string(out), "Monitor is On"), nil
}

func (d *Driver) Reset(ctx context.Context) error {
	hostname, err := envdriver.GetHostname(ctx)
	if err != nil {
		return err
	}
	if hostname == "riverwood" {
		// Ensure eDP-1 is primary and on the left, HDMI-A-1 on the right
		return execdriver.MustRun(ctx, "xrandr",
			"--output", "eDP-1", "--auto", "--primary", "--pos", "0x0",
			"--output", "HDMI-A-1", "--auto", "--right-of", "eDP-1",
		).Run()
	}
	if hostname == "whiterun" {
		return execdriver.MustRun(ctx, "xrandr", "--output", "HDMI-1", "--auto").Run()
	}
	return api.ErrNotImplemented
}
