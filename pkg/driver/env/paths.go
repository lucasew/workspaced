package env

import (
	"context"
	"os"
	"path/filepath"

	"workspaced/pkg/api"
	"workspaced/pkg/constants"
)

// FindDotfilesRoot walks constants.DotfilesCandidates using home for ~ expansion.
func FindDotfilesRoot(home string) (string, error) {
	for _, path := range constants.DotfilesCandidates {
		expanded := ExpandPathIn(path, home)
		if info, err := os.Stat(expanded); err == nil && info.IsDir() {
			return expanded, nil
		}
	}
	return "", api.ErrDotfilesRootNotFound
}

// EnsureUnderHome joins home/rel, creates the directory, and returns it.
func EnsureUnderHome(home, rel string) (string, error) {
	path := filepath.Join(home, rel)
	if err := os.MkdirAll(path, 0o755); err != nil {
		return "", err
	}
	return path, nil
}

// Hostname is os.Hostname with a stable error path for drivers.
func Hostname(_ context.Context) (string, error) {
	return os.Hostname()
}
