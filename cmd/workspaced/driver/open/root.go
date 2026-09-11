package open

import (
	"context"

	"github.com/lewtec/lewkit/x/cmd"
	"github.com/lucasew/workspaced/pkg/driver/opener"
)

type Command struct {
	target cmd.StringArg
}

func (Command) Description() string {
	return "Open a file or URL using the preferred opener"
}

func (c *Command) Run(ctx context.Context) error {
	return opener.Open(ctx, c.target.Value())
}
