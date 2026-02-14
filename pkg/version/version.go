package version

import (
	_ "embed"
	"strings"
)

//go:embed version.txt
var versionFile string

// Version returns the current workspaced version
func Version() string {
	return strings.TrimSpace(versionFile)
}

// BuildID returns the current workspaced version (alias for Version)
func BuildID() string {
	return Version()
}

// GetBuildID returns the current workspaced version (alias for Version)
func GetBuildID() string {
	return Version()
}
