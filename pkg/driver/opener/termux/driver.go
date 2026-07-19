package termux

import (
	"context"

	"github.com/lucasew/workspaced/pkg/driver"
	"github.com/lucasew/workspaced/pkg/driver/opener"
)

func init() {
	opener.RegisterBinary("opener_termux", "termux-open", "termux-open", func(context.Context) error {
		return driver.RequireTermux()
	})
}
