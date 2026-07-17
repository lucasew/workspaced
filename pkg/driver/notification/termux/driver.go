package termux

import (
	"context"
	"fmt"
	"workspaced/pkg/driver"
	execdriver "workspaced/pkg/driver/exec"
	"workspaced/pkg/driver/notification"
)

func init() {
	driver.Register[notification.Driver](&Factory{})
}

type Factory struct{}

func (p *Factory) ID() string   { return "notification_termux" }
func (p *Factory) Name() string { return "Termux" }

func (p *Factory) CheckCompatibility(ctx context.Context) error {
	if !execdriver.IsBinaryAvailable(ctx, "termux-notification") {
		return fmt.Errorf("%w: termux-notification not found", driver.ErrIncompatible)
	}
	return nil
}

func (p *Factory) New(ctx context.Context) (notification.Driver, error) {
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
