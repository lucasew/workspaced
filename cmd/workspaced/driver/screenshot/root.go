package screenshot

import (
	"context"

	"github.com/lucasew/workspaced/internal/clirun"
)

type Command struct {
	All    *All
	Full   *Full
	Output *Output
	Window *Window
	Select *Select
}

func (Command) Description() string {
	return "Screen capture management"
}

func (c *Command) Run(ctx context.Context) error {
	return clirun.PrintUsage[Command]("workspaced driver screenshot")
}
