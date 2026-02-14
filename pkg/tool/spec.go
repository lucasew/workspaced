package tool

import (
	"fmt"
	"strings"
)

// ToolSpec represents a parsed tool specification
type ToolSpec struct {
	Provider string // e.g., "github"
	Package  string // e.g., "denoland/deno"
	Version  string // e.g., "1.40.0" or "latest"
}

// String returns the canonical string representation of the tool spec
func (ts ToolSpec) String() string {
	return fmt.Sprintf("%s:%s@%s", ts.Provider, ts.Package, ts.Version)
}

// Dir returns the directory name for this tool spec
// Example: ToolSpec{Provider: "github", Package: "denoland/deno"} -> "github-denoland-deno"
func (ts ToolSpec) Dir() string {
	return SpecToDir(ts.Provider, ts.Package)
}

// ParseToolSpec parses a tool specification string.
//
// Formats supported:
//   - provider:package@version -> full spec
//   - provider:package         -> defaults version to "latest"
//   - package@version          -> defaults provider to "registry" (not yet implemented - use explicit provider)
//   - package                  -> defaults provider to "registry" and version to "latest"
//
// Examples:
//   - "github:denoland/deno@1.40.0" -> ToolSpec{Provider: "github", Package: "denoland/deno", Version: "1.40.0"}
//   - "github:denoland/deno"        -> ToolSpec{Provider: "github", Package: "denoland/deno", Version: "latest"}
//   - "deno@1.40.0"                 -> ToolSpec{Provider: "registry", Package: "deno", Version: "1.40.0"} (will fail - not implemented)
//   - "deno"                        -> ToolSpec{Provider: "registry", Package: "deno", Version: "latest"} (will fail - not implemented)
func ParseToolSpec(spec string) (ToolSpec, error) {
	if spec == "" {
		return ToolSpec{}, fmt.Errorf("tool spec cannot be empty")
	}

	const defaultProvider = "registry" // Placeholder for future registry implementation
	const defaultVersion = "latest"

	// Check if provider is specified (contains ':')
	var providerID, rest string
	if strings.Contains(spec, ":") {
		parts := strings.SplitN(spec, ":", 2)
		providerID = parts[0]
		rest = parts[1]
	} else {
		// No provider specified, use default
		providerID = defaultProvider
		rest = spec
	}

	// Parse package@version
	parts := strings.SplitN(rest, "@", 2)
	pkg := parts[0]
	version := defaultVersion
	if len(parts) == 2 {
		version = parts[1]
	}

	return ToolSpec{
		Provider: providerID,
		Package:  pkg,
		Version:  version,
	}, nil
}

// SpecToDir normalizes spec to directory name: "github:denoland/deno" -> "github-denoland-deno"
func SpecToDir(providerID, pkgSpec string) string {
	s := fmt.Sprintf("%s-%s", providerID, pkgSpec)
	s = strings.ReplaceAll(s, "/", "-")
	s = strings.ReplaceAll(s, ":", "-")
	return s
}
