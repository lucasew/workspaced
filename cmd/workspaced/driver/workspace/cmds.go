package workspace

import (
	"context"

	"github.com/lewtec/lewkit/x/cmd"
	"github.com/lucasew/workspaced/pkg/driver/wm"
)

type Rotate struct {
	Move cmd.Flag `long:"move" help:"Move container to workspace"`
}

func (Rotate) Description() string { return "Rotate workspaces across outputs" }

func (*Rotate) Run(ctx context.Context) error {
	return wm.RotateWorkspaces(ctx)
}

type Scratchpad struct {
	Move cmd.Flag `long:"move" help:"Move container to workspace"`
}

func (Scratchpad) Description() string { return "Toggle scratchpad visibility with status info" }

func (*Scratchpad) Run(ctx context.Context) error {
	return wm.ToggleScratchpadWithInfo(ctx)
}

type Next struct {
	Move cmd.Flag `long:"move" help:"Move container to workspace"`
}

func (Next) Description() string { return "Go to the next available workspace" }

func (c *Next) Run(ctx context.Context) error {
	return wm.NextWorkspace(ctx, c.Move.Value())
}
