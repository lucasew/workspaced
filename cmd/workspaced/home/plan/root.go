package plan

import (
	"context"

	"github.com/lewtec/lewkit/x/cmd"
	"github.com/lucasew/workspaced/cmd/workspaced/home/apply"
	"github.com/lucasew/workspaced/internal/cmdwire"
)

type Command struct {
	ShowNoop cmd.Flag `long:"show-noop" help:"Also show files that would not change"`
}

func (Command) Description() string {
	return "Show what would be applied (dry-run)"
}

func (c *Command) Run(ctx context.Context) error {
	return cmdwire.RunAfterWait(ctx, true, c.ShowNoop.Value(), apply.Schedule)
}
