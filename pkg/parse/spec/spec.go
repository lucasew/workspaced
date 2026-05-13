package spec

import (
	"fmt"
	"strings"
)

type Spec struct {
	Provider string
	Package  string
	Version  string
}

func (s Spec) Dir() string {
	return ToDir(s.Provider, s.Package)
}

func (s Spec) String() string {
	return fmt.Sprintf("%s:%s@%s", s.Provider, s.Package, s.Version)
}

func Parse(input string) (Spec, error) {
	if input == "" {
		return Spec{}, fmt.Errorf("spec cannot be empty")
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

func ToDir(providerID, pkgSpec string) string {
	s := fmt.Sprintf("%s-%s", providerID, pkgSpec)
	s = strings.ReplaceAll(s, "/", "-")
	s = strings.ReplaceAll(s, ":", "-")
	return s
}
