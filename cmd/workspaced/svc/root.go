package svc

import (
	"context"

	"github.com/lucasew/workspaced/internal/clirun"
)

type Command struct {
	Osmardetector *Osmardetector
	ReniceHungry  *ReniceHungry `cmd:"renice-hungry"`
	Screencaps    *Screencaps
	Vncd          *Vncd
}

func (Command) Description() string {
	return "Background services"
}

func (c *Command) Run(ctx context.Context) error {
	return clirun.PrintUsage[Command]("workspaced svc")
}
