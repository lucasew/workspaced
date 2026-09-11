package config

import (
	"context"

	"github.com/lucasew/workspaced/cmd/workspaced/configcmd"
	"github.com/lucasew/workspaced/internal/clirun"
)

type Command struct {
	configcmd.Tree[configcmd.Codebase] `flatten:""`
}

func (Command) Description() string { return "Manage configuration" }

func (c *Command) Run(ctx context.Context) error {
	return clirun.PrintUsage[Command]("workspaced codebase config")
}
