package xdg

import (
	"context"

	"workspaced/pkg/driver"
	"workspaced/pkg/driver/opener"
)

func init() {
	opener.RegisterBinary("opener_xdg", "xdg-open", "xdg-open", func(ctx context.Context) error {
		return driver.RequireAnyEnv(ctx, "DISPLAY", "WAYLAND_DISPLAY")
	})
}
