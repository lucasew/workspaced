package env

import (
	"context"
	"os"
	"path/filepath"
	"workspaced/pkg/driver"
)

// Driver provides platform-specific environment operations.
type Driver interface {
	// GetDotfilesRoot locates the root directory of the dotfiles repository.
	GetDotfilesRoot(ctx context.Context) (string, error)

	// GetHostname returns the current system hostname.
	GetHostname(ctx context.Context) (string, error)

	// GetUserDataDir returns the path to the user data directory for workspaced.
	GetUserDataDir(ctx context.Context) (string, error)

	// GetConfigDir returns the path to the user config directory for workspaced.
	GetConfigDir(ctx context.Context) (string, error)

	// GetHomeDir returns the actual user home directory (handles Termux chroot)
	GetHomeDir(ctx context.Context) (string, error)

	// IsPhone checks if the environment suggests we are running on a phone.
	IsPhone(ctx context.Context) bool

	// IsNixOS checks if the system is NixOS.
	IsNixOS(ctx context.Context) bool

	// GetEssentialPaths returns platform-specific essential PATH directories.
	GetEssentialPaths(ctx context.Context) []string
}

// Facade functions

// GetDotfilesRoot locates the root directory of the dotfiles repository.
func GetDotfilesRoot(ctx context.Context) (string, error) {
	d, err := driver.Get[Driver](ctx)
	if err != nil {
		return "", err
	}
	return d.GetDotfilesRoot(ctx)
}

// GetHostname returns the current system hostname.
func GetHostname(ctx context.Context) (string, error) {
	d, err := driver.Get[Driver](ctx)
	if err != nil {
		return "", err
	}
	return d.GetHostname(ctx)
}

// GetUserDataDir returns the path to the user data directory for workspaced.
func GetUserDataDir(ctx context.Context) (string, error) {
	d, err := driver.Get[Driver](ctx)
	if err != nil {
		return "", err
	}
	return d.GetUserDataDir(ctx)
}

// GetConfigDir returns the path to the user config directory for workspaced.
func GetConfigDir(ctx context.Context) (string, error) {
	d, err := driver.Get[Driver](ctx)
	if err != nil {
		return "", err
	}
	return d.GetConfigDir(ctx)
}

// GetHomeDir returns the actual user home directory (handles Termux chroot).
func GetHomeDir(ctx context.Context) (string, error) {
	d, err := driver.Get[Driver](ctx)
	if err != nil {
		return "", err
	}
	return d.GetHomeDir(ctx)
}

// IsPhone checks if the environment suggests we are running on a phone.
func IsPhone(ctx context.Context) bool {
	d, err := driver.Get[Driver](ctx)
	if err != nil {
		return false
	}
	return d.IsPhone(ctx)
}

// IsNixOS checks if the system is NixOS.
func IsNixOS(ctx context.Context) bool {
	d, err := driver.Get[Driver](ctx)
	if err != nil {
		return false
	}
	return d.IsNixOS(ctx)
}

// GetEssentialPaths returns platform-specific essential PATH directories.
func GetEssentialPaths(ctx context.Context) []string {
	d, err := driver.Get[Driver](ctx)
	if err != nil {
		return nil
	}
	return d.GetEssentialPaths(ctx)
}

// IsInStore checks if the dotfiles root is located inside the Nix store.
func IsInStore(ctx context.Context) bool {
	root, err := GetDotfilesRoot(ctx)
	if err != nil {
		return false
	}
	return len(root) >= 10 && root[:10] == "/nix/store"
}

// ExpandPath expands ~ to home directory and environment variables in a path.
// Examples:
//   - "~/.config" -> "/home/user/.config"
//   - "$HOME/bin" -> "/home/user/bin"
//   - "~/file with spaces" -> "/home/user/file with spaces"
func ExpandPath(path string) string {
	if len(path) > 0 && path[0] == '~' {
		if home, err := os.UserHomeDir(); err == nil {
			if len(path) == 1 {
				return home
			}
			if path[1] == '/' {
				return filepath.Join(home, path[2:])
			}
		}
	}
	return os.ExpandEnv(path)
}
