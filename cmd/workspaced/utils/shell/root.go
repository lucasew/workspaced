package shell

import (
	"context"

	"github.com/lucasew/workspaced/internal/clirun"
)

type Command struct {
	Init *Init
}

func (Command) Description() string {
	return "Shell integration commands"
}

func (c *Command) Run(ctx context.Context) error {
	return clirun.PrintUsage[Command]("workspaced utils shell")
}
