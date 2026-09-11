package backup

import (
	"context"

	"github.com/lucasew/workspaced/internal/clirun"
)

type Command struct {
	RunCmd *Run `cmd:"run"`
}

func (Command) Description() string {
	return "Data backup and synchronization"
}

func (c *Command) Run(ctx context.Context) error {
	return clirun.PrintUsage[Command]("workspaced home backup")
}
