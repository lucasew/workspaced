package termux

import (
	"context"
	"os"
	"path/filepath"

	"github.com/lucasew/workspaced/pkg/driver"
	envdriver "github.com/lucasew/workspaced/pkg/driver/env"
)

func init() {
	driver.Register[envdriver.Driver](&Factory{})
}

type Factory struct{}

func (f *Factory) ID() string   { return "env_termux" }
func (f *Factory) Name() string { return "Termux Environment" }

func (f *Factory) CheckCompatibility(ctx context.Context) error {
	return driver.RequireTermux()
}

func (f *Factory) New(ctx context.Context) (envdriver.Driver, error) { return &Driver{}, nil }

type Driver struct{}

func (d *Driver) GetHomeDir(ctx context.Context) (string, error) {
	return envdriver.ResolveHomeDir()
}

func (d *Driver) GetDotfilesRoot(ctx context.Context) (string, error) {
	home, err := d.GetHomeDir(ctx)
	if err != nil {
		return "", err
	}
	return envdriver.FindDotfilesRoot(home)
}

func (d *Driver) GetHostname(ctx context.Context) (string, error) {
	return envdriver.Hostname(ctx)
}

func (d *Driver) GetUserDataDir(ctx context.Context) (string, error) {
	home, err := d.GetHomeDir(ctx)
	if err != nil {
		return "", err
	}
	return envdriver.EnsureUnderHome(home, ".local/share/workspaced")
}

func (d *Driver) GetConfigDir(ctx context.Context) (string, error) {
	home, err := d.GetHomeDir(ctx)
	if err != nil {
		return "", err
	}
	return envdriver.EnsureUnderHome(home, ".config/workspaced")
}

func (d *Driver) IsPhone(ctx context.Context) bool { return true }

func (d *Driver) IsNixOS(ctx context.Context) bool { return false }

func (d *Driver) GetEssentialPaths(ctx context.Context) []string {
	prefix := os.Getenv("PREFIX")
	if prefix == "" {
		prefix = "/data/data/com.termux/files/usr"
	}
	paths := []string{filepath.Join(prefix, "bin")}
	if home, err := d.GetHomeDir(ctx); err == nil {
		paths = append(paths, filepath.Join(home, ".local/bin"))
	}
	if dataDir, err := d.GetUserDataDir(ctx); err == nil {
		paths = append(paths, filepath.Join(dataDir, "shim/global"))
	}
	return paths
}
