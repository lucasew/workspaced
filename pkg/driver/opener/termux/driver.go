package termux

import (
	"context"

	"workspaced/pkg/driver"
	"workspaced/pkg/driver/opener"
)

func init() {
	opener.RegisterBinary("opener_termux", "termux-open", "termux-open", func(context.Context) error {
		return driver.RequireTermux()
	})
}
