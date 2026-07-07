package checks

import (
	"fmt"
	"os"
	"path/filepath"
)

// FindBinary searches for a binary named cmdName in the standard candidate
// locations under baseDir (bin/ and root, with .exe/.cmd/.bat variants).
// Returns the first existing path or empty string if none found.
func FindBinary(baseDir, cmdName string) string {
	for _, path := range BinaryCandidates(baseDir, cmdName) {
		if _, err := os.Stat(path); err == nil {
			return path
		}
	}
	return ""
}

// BinaryCandidates returns the list of candidate paths for a binary in
// the standard layout under baseDir.
func BinaryCandidates(baseDir, cmdName string) []string {
	exts := []string{"", ".exe", ".cmd", ".bat"}
	out := make([]string, 0, 2*len(exts))
	for _, dir := range []string{
		filepath.Join(baseDir, "bin"),
		baseDir,
	} {
		for _, ext := range exts {
			out = append(out, filepath.Join(dir, cmdName+ext))
		}
	}
	return out
}

// EnsureBinary runs install, then locates cmdName under destDir via FindBinary.
// toolLabel appears in the not-found error (e.g. "CMake", "Ruby").
func EnsureBinary(destDir, cmdName, toolLabel string, install func() error) (string, error) {
	if err := install(); err != nil {
		return "", err
	}
	if p := FindBinary(destDir, cmdName); p != "" {
		return p, nil
	}
	return "", fmt.Errorf("binary %q not found in %s installation at %s", cmdName, toolLabel, destDir)
}
