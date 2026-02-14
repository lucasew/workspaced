package bash

import (
	"context"
	"os"
	"strings"
	"workspaced/pkg/driver"
	execdriver "workspaced/pkg/driver/exec"
	shelldriver "workspaced/pkg/driver/shell"
)

type Provider struct{}

func (p *Provider) ID() string {
	return "shell_bash"
}

func (p *Provider) Name() string {
	return "Bash"
}

func (p *Provider) DefaultWeight() int {
	// Base weight: 50
	// If $SHELL points to bash, increase to 75
	userShell := os.Getenv("SHELL")
	if strings.Contains(userShell, "bash") {
		return 75
	}
	return 50
}

func (p *Provider) CheckCompatibility(ctx context.Context) error {
	_, err := execdriver.Which(ctx, "bash")
	return err
}

func (p *Provider) New(ctx context.Context) (shelldriver.Driver, error) {
	return &Driver{}, nil
}

type Driver struct{}

func (d *Driver) ID() string {
	return "shell_bash"
}

func (d *Driver) Name() string {
	return "Bash"
}

func (d *Driver) DefaultWeight() int {
	userShell := os.Getenv("SHELL")
	if strings.Contains(userShell, "bash") {
		return 75
	}
	return 50
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
	driver.Register[shelldriver.Driver](&Provider{})
}
