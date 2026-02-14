package version

import (
	_ "embed"
	"runtime/debug"
	"strings"
)

//go:embed version.txt
var versionTxt string

// Version returns the workspaced version from version.txt
func Version() string {
	return strings.TrimSpace(versionTxt)
}

// BuildID returns the build ID from buildinfo
func BuildID() string {
	info, ok := debug.ReadBuildInfo()
	if !ok {
		return "unknown"
	}

	// Try to get vcs.revision for commit hash
	for _, setting := range info.Settings {
		if setting.Key == "vcs.revision" {
			if len(setting.Value) > 8 {
				return setting.Value[:8] // short hash
			}
			return setting.Value
		}
	}

	return "dev"
}

// GetBuildID returns a build identifier combining version and commit hash
func GetBuildID() string {
	return Version() + "-" + BuildID()
}
