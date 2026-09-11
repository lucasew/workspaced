package nix

import (
	"context"
	"errors"

	"github.com/lucasew/workspaced/internal/clirun"
)

var (
	ErrNoFlakeRef    = errors.New("no flake reference provided")
	ErrNoBinaryFound = errors.New("no binary found")
)

type Command struct {
	Build     *Build
	Deploy    *Deploy
	GcCleanup *GcCleanup `cmd:"gc-cleanup"`
	Rbuild    *Rbuild
	Rrun      *Rrun
	RunCmd    *Run `cmd:"run"`
}

func (Command) Description() string {
	return "Nix operations"
}

func (c *Command) Run(ctx context.Context) error {
	return clirun.PrintUsage[Command]("workspaced utils nix")
}
