package input

import (
	"context"

	"github.com/lucasew/workspaced/pkg/driver"
	"github.com/lucasew/workspaced/pkg/driver/dialog"
)

type Launch struct{}

func (Launch) Description() string { return "Application launcher" }

func (*Launch) Run(ctx context.Context) error {
	d, err := driver.Get[dialog.Driver](ctx)
	if err != nil {
		return err
	}
	return d.RunApp(ctx)
}

type Window struct{}

func (Window) Description() string { return "Window switcher" }

func (*Window) Run(ctx context.Context) error {
	d, err := driver.Get[dialog.Driver](ctx)
	if err != nil {
		return err
	}
	return d.SwitchWindow(ctx)
}
