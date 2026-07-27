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

type Driver struct{ envdriver.Base }

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
