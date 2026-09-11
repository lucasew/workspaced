package system

import (
	"context"

	"github.com/lucasew/workspaced/internal/clirun"
)

type Command struct {
	Apply *Apply
}

func (Command) Description() string {
	return "System apply tools"
}

func (c *Command) Run(ctx context.Context) error {
	return clirun.PrintUsage[Command]("workspaced system")
}
