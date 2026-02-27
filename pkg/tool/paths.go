package tool

import (
	"os"
	"path/filepath"
)

// GetToolsDir returns the base directory where installed tools are stored.
// Defaults to `~/.local/share/workspaced/tools`.
func GetToolsDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".local", "share", "workspaced", "tools"), nil
}

// GetShimsDir returns the directory where tool shims are generated.
// Defaults to `~/.local/share/workspaced/shims`.
func GetShimsDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".local", "share", "workspaced", "shims"), nil
}
