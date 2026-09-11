package mod

import (
	"context"

	"github.com/lucasew/workspaced/internal/clirun"
	_ "github.com/lucasew/workspaced/internal/modfile/sourceprovider/prelude"
)

type Command struct {
	Lock *Lock
	Tidy *Tidy
}

func (Command) Description() string {
	return "Manage module sources and lockfile"
}

func (c *Command) Run(ctx context.Context) error {
	return clirun.PrintUsage[Command]("workspaced mod")
}
