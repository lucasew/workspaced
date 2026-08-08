package audio

import (
	"context"

	"workspaced/pkg/driver"
)

const step = 0.05

func IncreaseVolume(ctx context.Context) error {
	return adjustVolume(ctx, step)
}

func DecreaseVolume(ctx context.Context) error {
	return adjustVolume(ctx, -step)
}

func adjustVolume(ctx context.Context, delta float64) error {
	d, err := driver.Get[Driver](ctx)
	if err != nil {
		return err
	}
	vol, err := d.GetVolume(ctx)
	if err != nil {
		return err
	}
	newVol := vol + delta
	if newVol > 1.0 {
		newVol = 1.0
	}
	if newVol < 0 {
		newVol = 0
	}
	if err := d.SetVolume(ctx, newVol); err != nil {
		return err
	}
	return ShowStatus(ctx)
}

func ToggleMute(ctx context.Context) error {
	d, err := driver.Get[Driver](ctx)
	if err != nil {
		return err
	}
	if err := d.ToggleMute(ctx); err != nil {
		return err
	}
	return ShowStatus(ctx)
}
