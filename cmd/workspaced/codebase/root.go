package codebase

import (
	"context"

	"github.com/lucasew/workspaced/internal/clirun"
)

type Command struct {
	children `flatten:""`
	Apply    *Apply
	Plan     *Plan
	Lint     *Lint
	Format   *Format
	Lsp      *Lsp
	CIStatus *CIStatus `cmd:"ci-status"`
}

func (Command) Description() string {
	return "Tools for analyzing and managing codebases"
}

func (c *Command) Run(ctx context.Context) error {
	return clirun.PrintUsage[Command]("workspaced codebase")
}
