package env

import (
	"context"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/lucasew/workspaced/pkg/driver"
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

	// GetHomeDir returns the actual user home directory.
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
	return driver.WithResult(ctx, func(d Driver) (string, error) { return d.GetDotfilesRoot(ctx) })
}

// GetHostname returns the current system hostname.
func GetHostname(ctx context.Context) (string, error) {
	return driver.WithResult(ctx, func(d Driver) (string, error) { return d.GetHostname(ctx) })
}

// GetUserDataDir returns the path to the user data directory for workspaced.
func GetUserDataDir(ctx context.Context) (string, error) {
	return driver.WithResult(ctx, func(d Driver) (string, error) { return d.GetUserDataDir(ctx) })
}

// GetConfigDir returns the path to the user config directory for workspaced.
func GetConfigDir(ctx context.Context) (string, error) {
	return driver.WithResult(ctx, func(d Driver) (string, error) { return d.GetConfigDir(ctx) })
}

// GetHomeDir returns the actual user home directory.
func GetHomeDir(ctx context.Context) (string, error) {
	return driver.WithResult(ctx, func(d Driver) (string, error) { return d.GetHomeDir(ctx) })
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
	paths, err := driver.WithResult(ctx, func(d Driver) ([]string, error) {
		return d.GetEssentialPaths(ctx), nil
	})
	if err != nil {
		return nil
	}
	return paths
}

// IsInStore checks if the dotfiles root is located inside the Nix store.
func IsInStore(ctx context.Context) bool {
	root, err := GetDotfilesRoot(ctx)
	if err != nil {
		return false
	}
	return len(root) >= 10 && root[:10] == "/nix/store"
}

// ExpandPath expands ~ via ResolveHomeDir and $VAR via os.ExpandEnv.
// Callers that know a driver-specific home should use ExpandPathIn instead.
func ExpandPath(path string) string {
	home, _ := ResolveHomeDir()
	return ExpandPathIn(path, home)
}

// ExpandPathIn expands ~ using home and $VAR via os.ExpandEnv.
// Empty home leaves a leading ~ unexpanded.
func ExpandPathIn(path, home string) string {
	if home != "" && strings.HasPrefix(path, "~") {
		if path == "~" {
			return home
		}
		if path[1] == '/' || path[1] == filepath.Separator {
			return filepath.Join(home, path[2:])
		}
	}
	return os.ExpandEnv(path)
}

// ExpandPathContext expands path using the active env driver's home directory.
func ExpandPathContext(ctx context.Context, path string) (string, error) {
	home, err := GetHomeDir(ctx)
	if err != nil {
		return "", err
	}
	return ExpandPathIn(path, home), nil
}

// NormalizeURL normalizes a URL by adding a protocol if missing.
// Absolute/tilde paths become file://; everything else gets https://.
func NormalizeURL(url string) string {
	if strings.HasPrefix(url, "file://") || strings.HasPrefix(url, "http://") || strings.HasPrefix(url, "https://") {
		return url
	}
	if strings.HasPrefix(url, "/") || strings.HasPrefix(url, "~/") {
		return "file://" + ExpandPath(url)
	}
	return "https://" + url
}

// SetupEssentialPaths adjusts the current process $PATH to include platform
// essential directories (from the selected env driver). Call from root command
// setup with a ctx that already has a logger.
func SetupEssentialPaths(ctx context.Context) {
	essential := GetEssentialPaths(ctx)
	existing := strings.Split(os.Getenv("PATH"), ":")
	if err := os.Setenv("PATH", strings.Join(MergeEssentialPaths(essential, existing), ":")); err != nil {
		panic(err)
	}
}

// MergeEssentialPaths prepends missing essential dirs onto existing PATH entries.
// Entries already present are skipped. Multiple missing essentials keep the
// historical per-item prepend order (last missing essential ends up first).
func MergeEssentialPaths(essential, existing []string) []string {
	seen := make(map[string]struct{}, len(existing)+len(essential))
	for _, p := range existing {
		seen[p] = struct{}{}
	}
	missing := make([]string, 0, len(essential))
	for _, path := range essential {
		if _, ok := seen[path]; ok {
			continue
		}
		seen[path] = struct{}{}
		missing = append(missing, path)
	}
	if len(missing) == 0 {
		return slices.Clone(existing)
	}
	slices.Reverse(missing)
	return append(missing, existing...)
}
