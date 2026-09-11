package palette

import (
	"context"

	"github.com/lucasew/workspaced/internal/clirun"
)

type Command struct {
	children `flatten:""`
}

func (Command) Description() string {
	return "Color palette generation and management"
}

func (c *Command) Run(ctx context.Context) error {
	return clirun.PrintUsage[Command]("workspaced utils palette")
}
