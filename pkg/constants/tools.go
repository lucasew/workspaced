package constants

// BinaryNameSuffixes is the list of suffixes to remove from binary names
// when normalizing tool names. Checked in order, first match is removed.
// Note: Version patterns like "-v1.0.0" or "-1.0.0" are handled separately
// by BinaryVersionPattern.
var BinaryNameSuffixes = []string{
	// OS-Arch combinations (most specific first)
	"-linux-amd64", "-linux-x86_64", "-linux-x64",
	"-linux-arm64", "-linux-aarch64",
	"-linux-386", "-linux-x86",
	"-darwin-amd64", "-darwin-x86_64", "-darwin-x64",
	"-darwin-arm64", "-darwin-aarch64",
	"-windows-amd64", "-windows-x86_64", "-windows-x64",
	"-windows-arm64",
	"-windows-386", "-windows-x86",
	// Just OS
	"-linux", "-darwin", "-macos", "-windows",
	// Just Arch
	"-amd64", "-x86_64", "-x64",
	"-arm64", "-aarch64",
	"-386", "-x86",
}

// BinaryVersionPattern matches version suffixes like "-v1.0.0" or "-1.0.0"
// This is a regex pattern that will be applied after checking suffixes.
const BinaryVersionPattern = `-(v?\d+\.[\d.]+\w*)$`
