package bash

import (
	"context"
	"workspaced/pkg/driver"
	execdriver "workspaced/pkg/driver/exec"
	shelldriver "workspaced/pkg/driver/shell"
)

type Factory struct{}

func (f *Factory) ID() string {
	return "shell_bash"
}

func (f *Factory) Name() string {
	return "Bash"
}

func (f *Factory) CheckCompatibility(ctx context.Context) error {
	_, err := execdriver.Which(ctx, "bash")
	return err
}

func (f *Factory) New(ctx context.Context) (shelldriver.Driver, error) {
	return &Driver{}, nil
}

type Driver struct{}

func (d *Driver) ID() string {
	return "shell_bash"
}

func (d *Driver) Name() string {
	return "Bash"
}

func (d *Driver) CheckCompatibility(ctx context.Context) error {
	return nil
}

func (d *Driver) New(ctx context.Context) (shelldriver.Driver, error) {
	return d, nil
}

func (d *Driver) Path(ctx context.Context) (string, error) {
	return execdriver.Which(ctx, "bash")
}

func init() {
	driver.Register[shelldriver.Driver](&Factory{})
}
