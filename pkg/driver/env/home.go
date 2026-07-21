package env

import (
	"os"
	"path/filepath"
	"strings"
)

// defaultTermuxPrefix is used when PREFIX is unset but other Termux markers are present.
const defaultTermuxPrefix = "/data/data/com.termux/files/usr"

// ResolveHomeDir returns the real user home directory for path layout (shims,
// tool store, self-install).
//
// On Termux, $HOME is often "/home" inside proot/termux-chroot (root is
// $PREFIX/..). Paths baked that way break outside proot. When Termux is
// detected, "/home" (and empty HOME) are rewritten to $PREFIX/../home.
func ResolveHomeDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		// UserHomeDir fails when HOME is unset and passwd lookup fails.
		// Still try Termux rewrite from PREFIX alone.
		if fixed, ok := termuxHomeFromPrefix(); ok {
			return fixed, nil
		}
		return "", err
	}
	return NormalizeHome(home), nil
}

// NormalizeHome rewrites known Termux chroot home views to absolute paths.
// Non-Termux environments are returned unchanged.
func NormalizeHome(home string) string {
	if !IsTermuxLike() {
		return home
	}
	if home == "" || home == "/home" {
		if fixed, ok := termuxHomeFromPrefix(); ok {
			return fixed
		}
	}
	return home
}

// IsTermuxLike reports whether the process looks like Termux (or workspaced
// proot-inside-Termux), so home path normalization should apply.
func IsTermuxLike() bool {
	if os.Getenv("TERMUX_VERSION") != "" {
		return true
	}
	if os.Getenv("TERMUX_APP_PACKAGE") != "" {
		return true
	}
	if os.Getenv("WORKSPACED_IN_PROOT") == "1" {
		return true
	}
	prefix := os.Getenv("PREFIX")
	return strings.HasPrefix(prefix, "/data/data/com.termux/files")
}

func termuxHomeFromPrefix() (string, bool) {
	prefix := os.Getenv("PREFIX")
	if prefix == "" {
		if !IsTermuxLike() {
			return "", false
		}
		prefix = defaultTermuxPrefix
	}
	// PREFIX is .../files/usr → home is .../files/home
	home := filepath.Join(filepath.Dir(prefix), "home")
	return home, true
}
