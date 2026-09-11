package open

import (
	"context"
	"fmt"

	"github.com/lewtec/lewkit/x/cmd"
	"github.com/lucasew/workspaced/internal/tool"
	_ "github.com/lucasew/workspaced/internal/tool/prelude"
	execdriver "github.com/lucasew/workspaced/pkg/driver/exec"
	"github.com/lucasew/workspaced/pkg/taskgroup"
)

type Lazy struct {
	Bin  cmd.StringArg `long:"bin" help:"Binary name to resolve inside the tool package"`
	Home cmd.Flag      `long:"home" help:"Resolve the lazy tool using the home/dotfiles workspace"`
	tool cmd.StringArg
	sep  cmd.Dash
	args []cmd.StringArg
}

func (Lazy) Description() string {
	return "Run a lazy tool resolved from home config and workspaced.lock.json"
}

func (l *Lazy) Run(ctx context.Context) error {
	toolName := l.tool.Value()
	binName := l.Bin.Value()
	if binName == "" {
		binName = toolName
	}
	return runLazyTool(ctx, l.Home.Value(), toolName, binName, cmd.Values(l.args))
}

func runLazyTool(ctx context.Context, homeMode bool, toolName, binName string, toolArgs []string) error {
	resolver := tool.ResolveLazyTool
	if homeMode {
		resolver = tool.ResolveHomeLazyTool
	}

	g := taskgroup.MustFromContext(ctx)
	g.Go("open:lazy:"+toolName, taskgroup.Control, func(ctx context.Context, s *taskgroup.Status) error {
		s.Update("resolving " + toolName)
		binPath, err := resolver(ctx, toolName, binName)
		if err != nil {
			return err
		}
		execCtx := context.WithoutCancel(ctx)
		c, err := execdriver.Run(execCtx, binPath, toolArgs...)
		if err != nil {
			return fmt.Errorf("create command: %w", err)
		}
		s.AfterWaitRun(c)
		return nil
	})
	return nil
}
