package tool

import (
	"os"
	"path/filepath"
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
// locations under baseDir: baseDir/bin/cmdName, baseDir/bin/cmdName.exe,
// baseDir/cmdName, baseDir/cmdName.exe. Returns the first existing path
// or empty string if none found.
func FindBinary(baseDir, cmdName string) string {
	for _, path := range BinaryCandidates(baseDir, cmdName) {
		if _, err := os.Stat(path); err == nil {
			return path
		}
	}
	return ""
}

// BinaryCandidates returns the list of candidate paths for a binary in
// the standard layout under baseDir. This is the pure function version
// useful when you need the list itself rather than the first match.
func BinaryCandidates(baseDir, cmdName string) []string {
	return []string{
		filepath.Join(baseDir, "bin", cmdName),
		filepath.Join(baseDir, "bin", cmdName+".exe"),
		filepath.Join(baseDir, cmdName),
		filepath.Join(baseDir, cmdName+".exe"),
	}
}
