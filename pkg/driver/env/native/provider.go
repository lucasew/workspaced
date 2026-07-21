package native

import (
	"context"
	"os"
	"path/filepath"

	"github.com/lucasew/workspaced/pkg/driver"
	envdriver "github.com/lucasew/workspaced/pkg/driver/env"
)

type Factory struct{}

func (f *Factory) ID() string   { return "env_native" }
func (f *Factory) Name() string { return "Native Environment" }

func (f *Factory) CheckCompatibility(ctx context.Context) error { return nil }

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

func (d *Driver) IsPhone(ctx context.Context) bool { return driver.IsTermux() }

func (d *Driver) IsNixOS(ctx context.Context) bool {
	_, err := os.Stat("/etc/NIXOS")
	return err == nil
}

func (d *Driver) GetEssentialPaths(ctx context.Context) []string {
	paths := []string{"/run/wrappers/bin", "/run/current-system/sw/bin"}
	if home, err := d.GetHomeDir(ctx); err == nil {
		paths = append(paths, filepath.Join(home, ".nix-profile/bin"), filepath.Join(home, ".local/bin"))
	}
	if root, err := d.GetDotfilesRoot(ctx); err == nil && root != "" {
		paths = append(paths, filepath.Join(root, "bin/shim"))
	}
	if dataDir, err := d.GetUserDataDir(ctx); err == nil && dataDir != "" {
		paths = append(paths, filepath.Join(dataDir, "shim/global"))
	}
	return paths
}

func init() {
	driver.Register[envdriver.Driver](&Factory{})
}
