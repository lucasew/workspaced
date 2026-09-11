package tool

import (
	"context"

	"github.com/lucasew/workspaced/internal/clirun"
	_ "github.com/lucasew/workspaced/internal/tool/prelude"
)

type Command struct {
	List      *List
	Install   *Install
	Latest    *Latest
	Versions  *Versions
	Search    *Search
	Which     *Which
	With      *With
	Artifacts *Artifacts
}

func (Command) Description() string {
	return "Manage development tools"
}

func (c *Command) Run(ctx context.Context) error {
	return clirun.PrintUsage[Command]("workspaced tool")
}
