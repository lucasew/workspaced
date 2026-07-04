package notification

import (
	"context"

	"workspaced/pkg/driver"
)

func Notify(ctx context.Context, n *Notification) error {
	return driver.With(ctx, func(d Driver) error { return d.Notify(ctx, n) })
}
