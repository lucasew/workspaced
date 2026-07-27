package brightness

import (
	"context"

	"github.com/lucasew/workspaced/pkg/driver"
)

const step = 0.05

func IncreaseBrightness(ctx context.Context) error {
	return adjustBrightness(ctx, step)
}

func DecreaseBrightness(ctx context.Context) error {
	return adjustBrightness(ctx, -step)
}

func adjustBrightness(ctx context.Context, delta float64) error {
	d, err := driver.Get[Driver](ctx)
	if err != nil {
		return err
	}
	status, err := d.Status(ctx)
	if err != nil {
		return err
	}
	newLevel := driver.Clamp01(status.Brightness + delta)
	if err := d.SetBrightness(ctx, newLevel); err != nil {
		return err
	}
	return ShowStatus(ctx)
}
