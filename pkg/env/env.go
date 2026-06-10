package env

import (
	"context"
	"os"
	"path/filepath"
	"slices"
	"strings"

	envdriver "workspaced/pkg/driver/env"
	_ "workspaced/pkg/driver/env/native" // Register native provider
)

// EssentialPaths defines the list of directories that must be present in the PATH
// Deprecated: Use envdriver.GetEssentialPaths(ctx) instead
var EssentialPaths []string

func init() {
	// Early process setup. This one has to use Background because package inits
	// run before main has created the root logging ctx.
	ctx := context.Background()

	// Get essential paths from driver
	EssentialPaths = envdriver.GetEssentialPaths(ctx)

	// Apply to current process PATH
	newPath := strings.Split(os.Getenv("PATH"), ":")

	for _, path := range EssentialPaths {
		if !slices.Contains(newPath, path) {
			newPath = append([]string{path}, newPath...)
		}
	}
	if err := os.Setenv("PATH", strings.Join(newPath, ":")); err != nil {
		panic(err)
	}
}

// GetDotfilesRoot locates the root directory of the dotfiles repository.
// Deprecated: Use envdriver.GetDotfilesRoot(ctx) instead
func GetDotfilesRoot(ctx context.Context) (string, error) {
	return envdriver.GetDotfilesRoot(ctx)
}

// GetHostname returns the current system hostname.
// Deprecated: Use envdriver.GetHostname(ctx) instead
func GetHostname(ctx context.Context) string {
	hostname, _ := envdriver.GetHostname(ctx)
	return hostname
}

// GetUserDataDir returns the path to the user data directory for workspaced (~/.local/share/workspaced)
// Deprecated: Use envdriver.GetUserDataDir(ctx) instead
func GetUserDataDir(ctx context.Context) (string, error) {
	return envdriver.GetUserDataDir(ctx)
}

// GetConfigDir returns the path to the user config directory for workspaced (~/.config/workspaced)
// Deprecated: Use envdriver.GetConfigDir(ctx) instead
func GetConfigDir(ctx context.Context) (string, error) {
	return envdriver.GetConfigDir(ctx)
}

// IsPhone checks if the environment suggests we are running on a phone.
// Deprecated: Use envdriver.IsPhone(ctx) instead
func IsPhone(ctx context.Context) bool {
	return envdriver.IsPhone(ctx)
}

// IsInStore checks if the dotfiles root is located inside the Nix store.
func IsInStore() bool {
	root, err := GetDotfilesRoot(context.Background())
	if err != nil {
		return false
	}
	return strings.HasPrefix(root, "/nix/store")
}

// IsNixOS checks if the system is NixOS by verifying the existence of /etc/NIXOS.
// Deprecated: Use envdriver.IsNixOS(ctx) instead
func IsNixOS(ctx context.Context) bool {
	return envdriver.IsNixOS(ctx)
}

// ExpandPath expands the tilde (~) to the user's home directory
// and expands environment variables (e.g. $HOME, ${VAR}) in the path.
func ExpandPath(path string) string {
	if strings.HasPrefix(path, "~/") {
		home, _ := os.UserHomeDir()
		return filepath.Join(home, path[2:])
	}
	return os.ExpandEnv(path)
}

// NormalizeURL normalizes a URL by adding protocol if missing.
func NormalizeURL(url string) string {
	if strings.HasPrefix(url, "file://") || strings.HasPrefix(url, "http://") || strings.HasPrefix(url, "https://") {
		return url
	}
	if strings.HasPrefix(url, "/") || strings.HasPrefix(url, "~/") {
		return "file://" + ExpandPath(url)
	}
	return "https://" + url
}
