package codebase

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/lewtec/lewkit/x/cmd"
	"github.com/lucasew/workspaced/internal/checks/formatter"
	"github.com/lucasew/workspaced/internal/git"
	"github.com/lucasew/workspaced/pkg/taskgroup"
)

type Format struct {
	path *cmd.StringArg
}

func (Format) Description() string {
	return "Format code in the repository (runs at git root)"
}

func (f *Format) Run(ctx context.Context) error {
	path, err := os.Getwd()
	if err != nil {
		return err
	}
	if f.path != nil {
		path = f.path.Value()
	}

	absPath, err := filepath.Abs(path)
	if err != nil {
		return err
	}

	root, err := git.GetRoot(ctx, absPath)
	if err != nil {
		return fmt.Errorf("find git root (format must run inside a git repo): %w", err)
	}

	g := taskgroup.MustFromContext(ctx)
	g.Go("codebase:format", taskgroup.Control, func(ctx context.Context, s *taskgroup.Status) error {
		s.Update("running formatters")
		return formatter.RunAll(ctx, root)
	})
	return nil
}
