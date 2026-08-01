package tool

import (
	"path/filepath"

	"github.com/lucasew/workspaced/internal/tool/checks"
	envdriver "github.com/lucasew/workspaced/pkg/driver/env"
)

func GetToolsDir() (string, error) {
	return workspacedShareDir("tools")
}

func GetShimsDir() (string, error) {
	return workspacedShareDir("shims")
}

func workspacedShareDir(leaf string) (string, error) {
	home, err := envdriver.ResolveHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".local", "share", "workspaced", leaf), nil
}

// FindBinary searches for a binary named cmdName in the standard candidate
// locations under baseDir. See checks.FindBinary.
func FindBinary(baseDir, cmdName string) string {
	return checks.FindBinary(baseDir, cmdName)
}

// BinaryCandidates returns the list of candidate paths for a binary in
// the standard layout under baseDir. See checks.BinaryCandidates.
func BinaryCandidates(baseDir, cmdName string) []string {
	return checks.BinaryCandidates(baseDir, cmdName)
}
