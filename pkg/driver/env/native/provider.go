package native

import (
	"context"
	"os"
	"path/filepath"
	"workspaced/pkg/api"
	"workspaced/pkg/driver"
	envdriver "workspaced/pkg/driver/env"
)

type Provider struct{}

func (p *Provider) ID() string {
	return "env_native"
}

func (p *Provider) Name() string {
	return "Native Environment"
}

func (p *Provider) DefaultWeight() int {
	return driver.DefaultWeight
}

func (p *Provider) CheckCompatibility(ctx context.Context) error {
	// Always compatible
	return nil
}

func (p *Provider) New(ctx context.Context) (envdriver.Driver, error) {
	return &Driver{}, nil
}

type Driver struct{}

func (d *Driver) GetDotfilesRoot(ctx context.Context) (string, error) {
	home, err := os.UserHomeDir()
	if err == nil {
		path := filepath.Join(home, ".dotfiles")
		if info, err := os.Stat(path); err == nil && info.IsDir() {
			return path, nil
		}
	}
	// Fallback to /etc/.dotfiles
	path := "/etc/.dotfiles"
	if info, err := os.Stat(path); err == nil && info.IsDir() {
		return path, nil
	}
	return "", api.ErrDotfilesRootNotFound
}

func (d *Driver) GetHostname(ctx context.Context) (string, error) {
	hostname, err := os.Hostname()
	if err != nil {
		return "", err
	}
	return hostname, nil
}

func (d *Driver) GetUserDataDir(ctx context.Context) (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	path := filepath.Join(home, ".local/share/workspaced")
	if err := os.MkdirAll(path, 0755); err != nil {
		return "", err
	}
	return path, nil
}

func (d *Driver) GetConfigDir(ctx context.Context) (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	dir := filepath.Join(home, ".config/workspaced")
	if err := os.MkdirAll(dir, 0755); err != nil {
		return "", err
	}
	return dir, nil
}

func (d *Driver) IsPhone(ctx context.Context) bool {
	return os.Getenv("TERMUX_VERSION") != ""
}

func (d *Driver) IsNixOS(ctx context.Context) bool {
	_, err := os.Stat("/etc/NIXOS")
	return err == nil
}

func (d *Driver) GetEssentialPaths(ctx context.Context) []string {
	paths := []string{"/run/wrappers/bin", "/run/current-system/sw/bin"}

	if home, err := os.UserHomeDir(); err == nil {
		paths = append(paths, filepath.Join(home, ".nix-profile/bin"))
		paths = append(paths, filepath.Join(home, ".local/bin"))
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
	driver.Register[envdriver.Driver](&Provider{})
}
