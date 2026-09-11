package sudo

import (
	"context"

	"github.com/lewtec/lewkit/x/cmd"
	"github.com/lucasew/workspaced/internal/sudo"
)

type Reject struct {
	slug cmd.StringArg
}

func (Reject) Description() string { return "Reject a pending command" }

func (c *Reject) Run(ctx context.Context) error {
	return sudo.Remove(c.slug.Value())
}
