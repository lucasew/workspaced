package termux

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"workspaced/pkg/api"
	"workspaced/pkg/driver"
	envdriver "workspaced/pkg/driver/env"
)

func init() {
	driver.Register[envdriver.Driver](&Provider{})
}

type Provider struct{}

func (p *Provider) ID() string         { return "env_termux" }
func (p *Provider) Name() string       { return "Termux Environment" }
func (p *Provider) DefaultWeight() int { return 60 } // Higher than native

func (p *Provider) CheckCompatibility(ctx context.Context) error {
	if os.Getenv("TERMUX_VERSION") == "" {
		return fmt.Errorf("%w: not running in Termux", driver.ErrIncompatible)
	}
	return nil
}

func (p *Provider) New(ctx context.Context) (envdriver.Driver, error) {
	return &Driver{}, nil
}

type Driver struct{}

func (d *Driver) GetDotfilesRoot(ctx context.Context) (string, error) {
	home, err := d.GetHomeDir(ctx)
	if err != nil {
		return "", err
	}

	path := filepath.Join(home, ".dotfiles")
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
	home, err := d.GetHomeDir(ctx)
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
	home, err := d.GetHomeDir(ctx)
	if err != nil {
		return "", err
	}
	dir := filepath.Join(home, ".config/workspaced")
	if err := os.MkdirAll(dir, 0755); err != nil {
		return "", err
	}
	return dir, nil
}

func (d *Driver) GetHomeDir(ctx context.Context) (string, error) {
	// In Termux, handle both chrooted and non-chrooted environments
	home := os.Getenv("HOME")

	// If HOME is /home (chrooted) or empty, use Termux default
	if home == "" || home == "/home" {
		prefix := os.Getenv("PREFIX")
		if prefix == "" {
			prefix = "/data/data/com.termux/files/usr"
		}
		// Home is one level up from PREFIX
		home = filepath.Join(filepath.Dir(prefix), "home")
	}

	return home, nil
}

func (d *Driver) IsPhone(ctx context.Context) bool {
	return true // Termux always runs on Android phones
}

func (d *Driver) IsNixOS(ctx context.Context) bool {
	return false // Termux is not NixOS
}

func (d *Driver) GetEssentialPaths(ctx context.Context) []string {
	prefix := os.Getenv("PREFIX")
	if prefix == "" {
		prefix = "/data/data/com.termux/files/usr"
	}

	paths := []string{
		filepath.Join(prefix, "bin"),
	}

	if home, err := d.GetHomeDir(ctx); err == nil {
		paths = append(paths, filepath.Join(home, ".local/bin"))
	}

	if dataDir, err := d.GetUserDataDir(ctx); err == nil {
		paths = append(paths, filepath.Join(dataDir, "shim/global"))
	}

	return paths
}
