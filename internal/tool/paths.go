package tool

import (
	"os"
	"path/filepath"

	"workspaced/internal/tool/checks"
)

func GetToolsDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".local", "share", "workspaced", "tools"), nil
}

func GetShimsDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".local", "share", "workspaced", "shims"), nil
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
