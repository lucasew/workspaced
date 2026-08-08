package spec

import (
	"errors"
	"fmt"
	"strings"
)

var (
	// ErrEmptySpec is returned when an empty spec string is provided.
	ErrEmptySpec = errors.New("spec cannot be empty")
)

// Spec is a parsed colon/at reference of the form [scheme:]package[@version].
//
// Scheme is intentionally named Provider in this shared parser: tool call sites
// treat it as a backend id (see pkg/tool), while module/source call sites treat
// it as a source provider id (see pkg/modfile). Bare names default to scheme
// "registry" and version "latest".
type Spec struct {
	Provider string
	Package  string
	Version  string
}

// Dir returns the on-disk directory key for this spec's scheme and package.
func (s Spec) Dir() string {
	return ToDir(s.Provider, s.Package)
}

// String renders the canonical spec form scheme:package@version.
func (s Spec) String() string {
	return fmt.Sprintf("%s:%s@%s", s.Provider, s.Package, s.Version)
}

// Parse splits a spec string into scheme, package, and version components.
func Parse(input string) (Spec, error) {
	if input == "" {
		return Spec{}, ErrEmptySpec
	}

	const defaultProvider = "registry"
	const defaultVersion = "latest"

	var providerID, rest string
	if strings.Contains(input, ":") {
		parts := strings.SplitN(input, ":", 2)
		providerID = parts[0]
		rest = parts[1]
	} else {
		providerID = defaultProvider
		rest = input
	}

	parts := strings.SplitN(rest, "@", 2)
	pkg := parts[0]
	version := defaultVersion
	if len(parts) == 2 {
		version = parts[1]
	}

	return Spec{
		Provider: providerID,
		Package:  pkg,
		Version:  version,
	}, nil
}

// ToDir builds a filesystem-safe directory name from a scheme id and package ref.
func ToDir(providerID, pkgSpec string) string {
	s := fmt.Sprintf("%s-%s", providerID, pkgSpec)
	s = strings.ReplaceAll(s, "/", "-")
	s = strings.ReplaceAll(s, ":", "-")
	return s
}
