package termux

import (
	"context"
	"fmt"
	"github.com/lucasew/workspaced/pkg/driver"
	execdriver "github.com/lucasew/workspaced/pkg/driver/exec"
	"github.com/lucasew/workspaced/pkg/driver/notification"
)

func init() {
	driver.Register[notification.Driver](&Factory{})
}

type Factory struct{}

func (f *Factory) ID() string   { return "notification_termux" }
func (f *Factory) Name() string { return "Termux" }

func (f *Factory) CheckCompatibility(ctx context.Context) error {
	if !execdriver.IsBinaryAvailable(ctx, "termux-notification") {
		return fmt.Errorf("%w: termux-notification not found", driver.ErrIncompatible)
	}
	return nil
}

func (f *Factory) New(ctx context.Context) (notification.Driver, error) {
	return &Driver{}, nil
}

type Driver struct{}

func (d *Driver) Notify(ctx context.Context, n *notification.Notification) error {
	args := []string{
		"--title", n.Title,
		"--content", n.Message,
	}
	if n.ID != 0 {
		args = append(args, "--id", fmt.Sprintf("%d", n.ID))
	}
	return execdriver.MustRun(ctx, "termux-notification", args...).Run()
}
