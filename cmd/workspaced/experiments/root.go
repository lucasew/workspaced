package experiments

import (
	"context"

	"github.com/lucasew/workspaced/internal/clirun"
)

type Command struct {
	children `flatten:""`
	Cue      *Cue `cmd:"cue"`
}

func (Command) Description() string {
	return "Experimental features and prototypes"
}

func (c *Command) Run(ctx context.Context) error {
	return clirun.PrintUsage[Command]("workspaced experiments")
}
