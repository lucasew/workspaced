package tool

import (
	"context"
	"fmt"
	"os"

	"github.com/lewtec/lewkit/x/cmd"
	"github.com/lucasew/workspaced/internal/tool"
	"github.com/lucasew/workspaced/pkg/taskgroup"
)

type Which struct {
	spec   cmd.StringArg
	binary cmd.StringArg
}

func (Which) Description() string {
	return "Print absolute path to a binary inside the (ensured) tool ref"
}

func (w *Which) Run(ctx context.Context) error {
	spec := w.spec.Value()
	binary := w.binary.Value()

	m, err := tool.NewManager()
	if err != nil {
		return err
	}

	g := taskgroup.MustFromContext(ctx)
	var binPath string
	g.Go("tool:which:"+spec+":"+binary, taskgroup.Control, func(ctx context.Context, s *taskgroup.Status) error {
		s.Update("ensuring " + spec)
		bp, err := m.EnsureInstalled(ctx, spec, binary)
		if err != nil {
			return err
		}
		binPath = bp
		return nil
	})
	taskgroup.MustSessionFrom(ctx).AfterWait(func() error {
		if binPath != "" {
			_, err := fmt.Fprintln(os.Stdout, binPath)
			return err
		}
		return nil
	})
	return nil
}
