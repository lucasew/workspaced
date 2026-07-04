package constants

// BinaryNameSuffixes is the list of suffixes to remove from binary names
// when normalizing tool names. Checked in order, first match is removed.
// Note: Version patterns like "-v1.0.0" or "-1.0.0" are handled separately
// by BinaryVersionPattern.
var BinaryNameSuffixes = []string{
	// OS-Arch combinations (most specific first) — dash, dot, and underscore separators
	"-linux-amd64", "-linux-x86_64", "-linux-x64",
	"-linux-arm64", "-linux-aarch64",
	"-linux-386", "-linux-x86",
	"_linux_amd64", "_linux_arm64", "_linux_x86_64", "_linux_x64",
	"_linux_386", "_linux_arm", "_linux_mips", "_linux_mips64", "_linux_mips64le",
	"_linux_mipsle", "_linux_s390x",
	".linux.amd64", ".linux.arm64", ".linux.x86_64", ".linux.x64",
	"-darwin-amd64", "-darwin-x86_64", "-darwin-x64",
	"-darwin-arm64", "-darwin-aarch64",
	"_darwin_amd64", "_darwin_arm64", "_darwin_386",
	".darwin.amd64", ".darwin.arm64", ".darwin.x86_64",
	"-windows-amd64", "-windows-x86_64", "-windows-x64",
	"-windows-arm64",
	"-windows-386", "-windows-x86",
	"_windows_amd64", "_windows_386",
	".windows.amd64", ".windows.arm64",
	"_freebsd_amd64", "_freebsd_386", "_freebsd_arm",
	"_netbsd_amd64", "_netbsd_386", "_netbsd_arm",
	"_openbsd_amd64", "_openbsd_386",
	// Just OS
	"-linux", "-darwin", "-macos", "-windows",
	"_linux", "_darwin", "_macos", "_windows",
	".linux", ".darwin", ".macos", ".windows",
	// Just Arch
	"-amd64", "-x86_64", "-x64",
	"-arm64", "-aarch64",
	"-386", "-x86",
	"_amd64", "_arm64", "_x86_64", "_x64", "_386",
	".amd64", ".arm64", ".x86_64", ".x64",
}

// BinaryVersionPattern matches version suffixes like "-v1.0.0" or "-1.0.0"
// This is a regex pattern that will be applied after checking suffixes.
const BinaryVersionPattern = `-(v?\d+\.[\d.]+\w*)$`
