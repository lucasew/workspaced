package screenshot

import (
	"context"
	"errors"
	"fmt"

	dapi "github.com/lucasew/workspaced/pkg/api"
	"github.com/lucasew/workspaced/pkg/driver/screenshot"
)

func capture(ctx context.Context, target screenshot.TargetType) error {
	path, err := screenshot.Capture(ctx, target)
	if err != nil {
		return err
	}
	fmt.Println(path)
	return nil
}

type All struct{}

func (All) Description() string { return "Capture all outputs" }

func (*All) Run(ctx context.Context) error {
	return capture(ctx, screenshot.TargetAll)
}

type Full struct{}

func (Full) Description() string { return "Capture full screen (all outputs)" }

func (*Full) Run(ctx context.Context) error {
	return capture(ctx, screenshot.TargetAll)
}

type Output struct{}

func (Output) Description() string { return "Capture current output (monitor)" }

func (*Output) Run(ctx context.Context) error {
	return capture(ctx, screenshot.TargetOutput)
}

type Window struct{}

func (Window) Description() string { return "Capture current window" }

func (*Window) Run(ctx context.Context) error {
	return capture(ctx, screenshot.TargetWindow)
}

type Select struct{}

func (Select) Description() string { return "Capture selected area" }

func (*Select) Run(ctx context.Context) error {
	path, err := screenshot.Capture(ctx, screenshot.TargetSelection)
	if err != nil {
		if errors.Is(err, dapi.ErrCanceled) {
			return nil
		}
		return err
	}
	if path != "" {
		fmt.Println(path)
	}
	return nil
}
