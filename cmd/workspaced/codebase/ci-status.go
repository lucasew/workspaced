package codebase

import (
	"context"
	"os"

	"github.com/lewtec/lewkit/x/cmd"
	"github.com/lucasew/workspaced/internal/git"
	"github.com/lucasew/workspaced/internal/tool"
)

type CIStatus struct {
	args []cmd.StringArg
}

func (CIStatus) Description() string {
	return "Run ci-status from the workspace lazy_tools pin"
}

func (c *CIStatus) Run(ctx context.Context) error {
	run, err := tool.EnsureAndRunLazy(ctx, "ci_status", "ci-status", cmd.Values(c.args)...)
	if err != nil {
		return err
	}
	wd, err := os.Getwd()
	if err != nil {
		return err
	}
	run.Dir, err = git.GetRoot(ctx, wd)
	if err != nil {
		return err
	}
	run.Stdin = os.Stdin
	run.Stdout = os.Stdout
	run.Stderr = os.Stderr
	return run.Run()
}
