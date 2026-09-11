package workspace

import (
	"context"

	"github.com/lewtec/lewkit/x/cmd"
	"github.com/lucasew/workspaced/internal/clirun"
)

type Command struct {
	Move       cmd.Flag `long:"move" help:"Move container to workspace"`
	Rotate     *Rotate
	Scratchpad *Scratchpad
	Next       *Next
}

func (Command) Description() string {
	return "Workspace management commands"
}

func (c *Command) Run(ctx context.Context) error {
	return clirun.PrintUsage[Command]("workspaced driver workspace")
}
