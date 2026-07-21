package shim

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/lucasew/workspaced/pkg/driver"
	envdriver "github.com/lucasew/workspaced/pkg/driver/env"
)

var (
	ErrEmptyCommand = errors.New("command cannot be empty")
	ErrEmptyName    = errors.New("shim name cannot be empty")
)

// Driver provides shim script generation capabilities
type Driver interface {
	// GenerateContent creates the shim script content
	GenerateContent(command []string) (string, error)

	// Generate creates an executable shim at the given path that runs the specified command
	// path: absolute path where the shim will be created (e.g., "/home/user/.local/bin/python")
	// command: the command to execute when the shim is called (e.g., ["python3", "-m", "venv"])
	Generate(ctx context.Context, path string, command []string) error
}

// Get returns the active shim driver
func Get(ctx context.Context) (Driver, error) {
	return driver.Get[Driver](ctx)
}

// GenerateContent creates shim script content using the active driver
func GenerateContent(ctx context.Context, command []string) (string, error) {
	return driver.WithResult(ctx, func(d Driver) (string, error) { return d.GenerateContent(command) })
}

// Generate creates a shim using the active driver
func Generate(ctx context.Context, path string, command []string) error {
	return driver.With(ctx, func(d Driver) error { return d.Generate(ctx, path, command) })
}

// LocalBinDir returns ~/.local/bin for the current user.
// Uses env.ResolveHomeDir so Termux proot HOME=/home is expanded to the real path.
func LocalBinDir() (string, error) {
	home, err := envdriver.ResolveHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".local", "bin"), nil
}

// GenerateInLocalBin ensures ~/.local/bin exists and writes an executable shim
// named name that runs command. Returns the absolute shim path. Callers own
// success logging so install vs update wording stays at the call site.
//
// command paths should already be absolute real paths (not Termux chroot views);
// self-install uses ResolveHomeDir for that reason.
func GenerateInLocalBin(ctx context.Context, name string, command []string) (string, error) {
	if name == "" {
		return "", ErrEmptyName
	}
	if len(command) == 0 {
		return "", ErrEmptyCommand
	}
	if filepath.Base(name) != name || name == "." || name == ".." {
		return "", fmt.Errorf("shim name %q must be a plain base name", name)
	}

	// Normalize absolute command targets so shims stay valid outside proot.
	normalized := make([]string, len(command))
	for i, arg := range command {
		if i == 0 && filepath.IsAbs(arg) {
			normalized[i] = normalizeAbsHomePath(arg)
		} else {
			normalized[i] = arg
		}
	}
	command = normalized

	localBin, err := LocalBinDir()
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(localBin, 0o755); err != nil {
		return "", err
	}

	shimPath := filepath.Join(localBin, name)
	if err := Generate(ctx, shimPath, command); err != nil {
		return "", err
	}
	return shimPath, nil
}

// normalizeAbsHomePath rewrites a Termux chroot absolute path that starts with
// /home/ to the real $PREFIX/../home/... so the baked exec target works outside proot.
func normalizeAbsHomePath(path string) string {
	home, err := envdriver.ResolveHomeDir()
	if err != nil || home == "" || home == "/home" {
		return path
	}
	const chrootHome = "/home"
	if path == chrootHome {
		return home
	}
	if strings.HasPrefix(path, chrootHome+"/") {
		return home + path[len(chrootHome):]
	}
	return path
}
