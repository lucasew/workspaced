package version

import "runtime/debug"

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

// GetBuildID is an alias for BuildID (deprecated)
func GetBuildID() string {
	return BuildID()
}
