package demo

import (
	"context"

	"github.com/lucasew/workspaced/internal/clirun"
)

type Command struct {
	Debug    *Debug
	Progress *Progress
}

func (Command) Description() string {
	return "Demo commands"
}

func (c *Command) Run(ctx context.Context) error {
	return clirun.PrintUsage[Command]("workspaced utils demo")
}
