package driver

import (
	"context"

	"github.com/lucasew/workspaced/internal/clirun"
)

type Command struct {
	children `flatten:""`
}

func (Command) Description() string {
	return "Commands to interact with drivers"
}

func (c *Command) Run(ctx context.Context) error {
	return clirun.PrintUsage[Command]("workspaced driver")
}
