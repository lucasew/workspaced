package screen

import (
	"context"

	"workspaced/pkg/driver/screen"
)

func init() {
	Registry.Add("on", "Turn on the screen (DPMS)", func(ctx context.Context) error {
		return screen.SetDPMS(ctx, true)
	})
	Registry.Add("off", "Turn off the screen (DPMS)", func(ctx context.Context) error {
		return screen.SetDPMS(ctx, false)
	})
	Registry.Add("toggle", "Toggle screen state (DPMS)", screen.ToggleDPMS)
	Registry.Add("reset", "Reset screen resolution based on host", screen.Reset)
}
