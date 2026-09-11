package template

import (
	"context"

	"github.com/lucasew/workspaced/internal/clirun"
)

type Command struct {
	Materialize *Materialize
}

func (Command) Description() string {
	return "Template management commands"
}

func (c *Command) Run(ctx context.Context) error {
	return clirun.PrintUsage[Command]("workspaced utils template")
}
