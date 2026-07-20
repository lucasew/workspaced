package version

import (
	"runtime"
	"runtime/debug"
	"strings"
)

var version = "dev"

// Version returns the workspaced version.
// It defaults to "dev" when ldflags injection is not provided.
func Version() string {
	v := strings.TrimSpace(version)
	if v == "" {
		return "dev"
	}
	return v
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

// Platform returns GOOS-GOARCH[-microarch] for this binary.
// Microarch comes from the build setting for the active GOARCH
// (GOAMD64, GOARM, GOARM64, GO386, …), e.g. "linux-amd64-v1", "linux-arm-7".
func Platform() string {
	p := runtime.GOOS + "-" + runtime.GOARCH
	if m := microarch(); m != "" {
		return p + "-" + m
	}
	return p
}

// microarch returns the GO$GOARCH microarchitecture level recorded at build time.
func microarch() string {
	info, ok := debug.ReadBuildInfo()
	if !ok {
		return ""
	}
	for _, s := range info.Settings {
		switch s.Key {
		case "GOAMD64", "GOARM", "GOARM64", "GO386",
			"GOMIPS", "GOMIPS64", "GOPPC64", "GORISCV64", "GOWASM":
			return s.Value
		}
	}
	return ""
}

// GetBuildID returns a build identifier combining version and commit hash.
// No platform: keep it filename-safe (shell-init cache keys).
func GetBuildID() string {
	return Version() + "-" + BuildID()
}

// VersionString is the full --version line body: "<buildID> <platform>".
func VersionString() string {
	return GetBuildID() + " " + Platform()
}
