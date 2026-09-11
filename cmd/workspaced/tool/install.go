package tool

import (
	"context"

	"github.com/lewtec/lewkit/x/cmd"
	"github.com/lucasew/workspaced/internal/tool"
	"github.com/lucasew/workspaced/pkg/taskgroup"
)

type Install struct {
	spec cmd.StringArg
}

func (Install) Description() string { return "Install a tool" }

func (i *Install) Run(ctx context.Context) error {
	manager, err := tool.NewManager()
	if err != nil {
		return err
	}

	spec := i.spec.Value()
	g := taskgroup.MustFromContext(ctx)
	g.Go("tool:install:"+spec, taskgroup.Control, func(ctx context.Context, s *taskgroup.Status) error {
		s.Update("installing " + spec)
		return manager.Install(ctx, spec)
	})
	return nil
}
