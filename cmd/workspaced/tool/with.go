package tool

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/lewtec/lewkit/x/cmd"
	"github.com/lucasew/workspaced/internal/tool"
	execdriver "github.com/lucasew/workspaced/pkg/driver/exec"
	"github.com/lucasew/workspaced/pkg/taskgroup"
)

type With struct {
	Tools []cmd.StringArg
	Sep   cmd.Dash
	Cmd   []cmd.StringArg
}

func (With) Description() string {
	return `Execute a command with specific tool version(s)

All <tool-spec> arguments before "--" are ensured (installed if missing).
The <command> is resolved from the first (rightmost) listed tool that provides a binary
of that name, so the backend that owns the command does not have to be the last entry.

If a tool is not installed, it will be installed automatically.

Tool spec format:
  backend:package@version  (full spec)
  backend:package          (uses latest version)
  package@version          (uses registry backend for curated short names)
  package                  (uses registry backend + latest; for curated short names e.g. ripgrep, uv)

Bare names (no backend:) default to the registry backend (curated github tools).
For mise-managed tools (e.g. go, node) or direct github use 'mise:' or 'github:'.

Examples:
  workspaced tool with github:denoland/deno@1.40.0 -- deno run app.ts
  workspaced tool with ripgrep -- rg pattern
  workspaced tool with uv -- uv --version
  workspaced tool with mise:go@1.21.0 -- go version
  workspaced tool with mise:go@1.21.0 mise:node@20 -- node --version
  workspaced tool with nodejs uv -- node --help`
}

func (w *With) Run(ctx context.Context) error {
	toolSpecs := cmd.Values(w.Tools)
	cmdLine := cmd.Values(w.Cmd)
	if len(toolSpecs) == 0 || len(cmdLine) == 0 {
		return fmt.Errorf("usage: workspaced tool with <tool-spec>... -- <command> [args...]")
	}

	command := cmdLine[0]
	commandArgs := cmdLine[1:]

	g := taskgroup.MustFromContext(ctx)
	g.Go("tool:with:"+strings.Join(toolSpecs, "+"), taskgroup.Control, func(ctx context.Context, s *taskgroup.Status) error {
		m, err := tool.NewManager()
		if err != nil {
			return err
		}

		type specItem struct {
			index int
			spec  string
		}
		type binOutcome struct {
			index   int
			binPath string
			miss    bool
		}
		items := make([]specItem, len(toolSpecs))
		for i, spec := range toolSpecs {
			items[i] = specItem{index: i, spec: spec}
		}

		outcomes, err := taskgroup.Map[specItem, binOutcome]{
			Name:     "tool-with:ensure",
			Items:    items,
			PoolKind: taskgroup.Control,
			TaskName: func(_ int, it specItem) string { return "ensure:" + it.spec },
			Fn: func(ctx context.Context, st *taskgroup.Status, it specItem) (binOutcome, error) {
				st.Update(it.spec)
				bp, err := m.EnsureInstalled(ctx, it.spec, command)
				if err == nil {
					return binOutcome{index: it.index, binPath: bp}, nil
				}
				if isBinaryNotFound(err) {
					return binOutcome{index: it.index, miss: true}, nil
				}
				return binOutcome{}, fmt.Errorf("ensure tool %s: %w", it.spec, err)
			},
		}.Run(ctx)
		if err != nil {
			return err
		}

		binPath := ""
		for i := len(outcomes) - 1; i >= 0; i-- {
			if !outcomes[i].miss && outcomes[i].binPath != "" {
				binPath = outcomes[i].binPath
				break
			}
		}
		if binPath == "" {
			return fmt.Errorf("none of the tools (%s) provide a binary named %q", strings.Join(toolSpecs, ", "), command)
		}

		execCtx := context.WithoutCancel(ctx)
		c, err := execdriver.Run(execCtx, binPath, commandArgs...)
		if err != nil {
			return err
		}
		s.AfterWaitRun(c)
		return nil
	})
	return nil
}

func isBinaryNotFound(err error) bool {
	return errors.Is(err, tool.ErrBinaryNotFound)
}
