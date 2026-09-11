package media

import (
	"context"

	"github.com/lucasew/workspaced/internal/clirun"
	"github.com/lucasew/workspaced/pkg/driver/media"
)

type Command struct {
	Next      *Next
	Previous  *Previous
	PlayPause *PlayPause `cmd:"play-pause"`
	Stop      *Stop
	Show      *Show
}

func (Command) Description() string {
	return "Control media playback"
}

func (c *Command) Run(ctx context.Context) error {
	return clirun.PrintUsage[Command]("workspaced driver media")
}

type Next struct{}

func (Next) Description() string { return "Next media" }
func (*Next) Run(ctx context.Context) error {
	return media.RunAction(ctx, "next")
}

type Previous struct{}

func (Previous) Description() string { return "Previous media" }
func (*Previous) Run(ctx context.Context) error {
	return media.RunAction(ctx, "previous")
}

type PlayPause struct{}

func (PlayPause) Description() string { return "Play or pause media" }
func (*PlayPause) Run(ctx context.Context) error {
	return media.RunAction(ctx, "play-pause")
}

type Stop struct{}

func (Stop) Description() string { return "Stop media" }
func (*Stop) Run(ctx context.Context) error {
	return media.RunAction(ctx, "stop")
}

type Show struct{}

func (Show) Description() string { return "Show media metadata" }
func (*Show) Run(ctx context.Context) error {
	return media.RunAction(ctx, "show")
}
