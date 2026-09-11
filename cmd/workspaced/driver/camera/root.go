package camera

import (
	"context"

	"github.com/lucasew/workspaced/internal/clirun"
)

type Command struct {
	List    *List
	Capture *Capture
}

func (Command) Description() string {
	return "Camera capture management"
}

func (c *Command) Run(ctx context.Context) error {
	return clirun.PrintUsage[Command]("workspaced driver camera")
}
