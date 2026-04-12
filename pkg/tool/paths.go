package tool

import (
	"os"
	"path/filepath"
)

// GetToolsDir determines the standard storage path for downloaded and extracted binaries.
// Usually resolves to ~/.local/share/workspaced/tools.
func GetToolsDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".local", "share", "workspaced", "tools"), nil
}

// GetShimsDir determines the standard storage path for dynamically generated executable wrappers.
// These wrappers allow managed tools to be invoked as if they were globally installed.
// Usually resolves to ~/.local/share/workspaced/shims.
func GetShimsDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".local", "share", "workspaced", "shims"), nil
}
