package sh

import (
	"context"
	"workspaced/pkg/driver"
	execdriver "workspaced/pkg/driver/exec"
	shelldriver "workspaced/pkg/driver/shell"
)

type Provider struct{}

func (p *Provider) ID() string {
	return "shell_sh"
}

func (p *Provider) Name() string {
	return "POSIX sh"
}

func (p *Provider) CheckCompatibility(ctx context.Context) error {
	_, err := execdriver.Which(ctx, "sh")
	return err
}

func (p *Provider) New(ctx context.Context) (shelldriver.Driver, error) {
	return &Driver{}, nil
}

type Driver struct{}

func (d *Driver) ID() string {
	return "shell_sh"
}

func (d *Driver) Name() string {
	return "POSIX sh"
}

func (d *Driver) CheckCompatibility(ctx context.Context) error {
	return nil
}

func (d *Driver) New(ctx context.Context) (shelldriver.Driver, error) {
	return d, nil
}

func (d *Driver) Path(ctx context.Context) (string, error) {
	return execdriver.Which(ctx, "sh")
}

func init() {
	driver.Register[shelldriver.Driver](&Provider{})
}
