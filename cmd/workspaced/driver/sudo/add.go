package sudo

import (
	"context"
	"fmt"

	"github.com/lewtec/lewkit/x/cmd"
	"github.com/lucasew/workspaced/internal/sudo"
	"github.com/lucasew/workspaced/internal/types"
)

type Add struct {
	Slug cmd.StringArg `short:"s" long:"slug" help:"Slug for the command"`
	sep  cmd.Dash
	rest []cmd.StringArg
}

func (Add) Description() string {
	return "Manually add a command to the queue. Use -- to separate flags from the command."
}

func (c *Add) Run(ctx context.Context) error {
	args := cmd.Values(c.rest)
	if len(args) == 0 {
		return fmt.Errorf("add requires a command after --")
	}
	sc := &types.SudoCommand{
		Slug:    c.Slug.Value(),
		Command: args[0],
		Args:    args[1:],
	}
	return sudo.Enqueue(ctx, sc)
}
