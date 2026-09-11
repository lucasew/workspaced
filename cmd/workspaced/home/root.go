package home

import (
	"context"

	"github.com/lucasew/workspaced/internal/clirun"
)

type Command struct {
	children `flatten:""`
}

func (Command) Description() string {
	return "Dotfiles and system state management"
}

func (c *Command) Run(ctx context.Context) error {
	return clirun.PrintUsage[Command]("workspaced home")
}
