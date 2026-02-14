package sh

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
	return "shell_sh"
}

func (p *Provider) Name() string {
	return "POSIX sh"
}

func (p *Provider) DefaultWeight() int {
	// Base weight: 50
	// If $SHELL points to sh (or dash, ash, etc), increase to 75
	userShell := os.Getenv("SHELL")
	if strings.HasSuffix(userShell, "/sh") ||
		strings.Contains(userShell, "dash") ||
		strings.Contains(userShell, "ash") {
		return 75
	}
	return 50
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

func (d *Driver) DefaultWeight() int {
	userShell := os.Getenv("SHELL")
	if strings.HasSuffix(userShell, "/sh") ||
		strings.Contains(userShell, "dash") ||
		strings.Contains(userShell, "ash") {
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
	return execdriver.Which(ctx, "sh")
}

func init() {
	driver.Register[shelldriver.Driver](&Provider{})
}
