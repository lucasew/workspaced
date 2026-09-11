package screen

import (
	"context"

	"github.com/lucasew/workspaced/internal/clirun"
	"github.com/lucasew/workspaced/pkg/driver/screen"
)

type Command struct {
	On     *On
	Off    *Off
	Toggle *Toggle
	Reset  *Reset
}

func (Command) Description() string {
	return "Screen and power management"
}

func (c *Command) Run(ctx context.Context) error {
	return clirun.PrintUsage[Command]("workspaced driver screen")
}

type On struct{}

func (On) Description() string { return "Turn on the screen (DPMS)" }
func (*On) Run(ctx context.Context) error {
	return screen.SetDPMS(ctx, true)
}

type Off struct{}

func (Off) Description() string { return "Turn off the screen (DPMS)" }
func (*Off) Run(ctx context.Context) error {
	return screen.SetDPMS(ctx, false)
}

type Toggle struct{}

func (Toggle) Description() string { return "Toggle screen state (DPMS)" }
func (*Toggle) Run(ctx context.Context) error {
	return screen.ToggleDPMS(ctx)
}

type Reset struct{}

func (Reset) Description() string { return "Reset screen resolution based on host" }
func (*Reset) Run(ctx context.Context) error {
	return screen.Reset(ctx)
}
