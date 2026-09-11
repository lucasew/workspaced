package sudo

import (
	"context"

	"github.com/lucasew/workspaced/internal/clirun"
)

type Command struct {
	Add     *Add
	Approve *Approve
	List    *List
	Reject  *Reject
}

func (Command) Description() string {
	return "Manage pending privileged commands"
}

func (c *Command) Run(ctx context.Context) error {
	return clirun.PrintUsage[Command]("workspaced driver sudo")
}
