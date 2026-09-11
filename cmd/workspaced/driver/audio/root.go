package audio

import (
	"context"

	"github.com/lucasew/workspaced/internal/clirun"
	"github.com/lucasew/workspaced/pkg/driver/audio"
)

type Command struct {
	Up     *Up
	Down   *Down
	Mute   *Mute
	Show   *Show
	Status *Show `cmd:"status"`
}

func (Command) Description() string {
	return "Control audio volume"
}

func (c *Command) Run(ctx context.Context) error {
	return clirun.PrintUsage[Command]("workspaced driver audio")
}

type Up struct{}

func (Up) Description() string { return "Increase volume" }
func (*Up) Run(ctx context.Context) error {
	return audio.IncreaseVolume(ctx)
}

type Down struct{}

func (Down) Description() string { return "Decrease volume" }
func (*Down) Run(ctx context.Context) error {
	return audio.DecreaseVolume(ctx)
}

type Mute struct{}

func (Mute) Description() string { return "Toggle mute" }
func (*Mute) Run(ctx context.Context) error {
	return audio.ToggleMute(ctx)
}

type Show struct{}

func (Show) Description() string { return "Show current volume" }
func (*Show) Run(ctx context.Context) error {
	return audio.ShowStatus(ctx)
}
