package icons

import (
	"context"

	"github.com/lucasew/workspaced/internal/clirun"
)

type Command struct {
	Generate *Generate
}

func (Command) Description() string {
	return "Icon theme generation utilities"
}

func (c *Command) Run(ctx context.Context) error {
	return clirun.PrintUsage[Command]("workspaced utils icons")
}
