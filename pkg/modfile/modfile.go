package modfile

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/BurntSushi/toml"
)

type SourceConfig struct {
	Provider string `toml:"provider"`
	Path     string `toml:"path"`
	Repo     string `toml:"repo"`
	URL      string `toml:"url"`
}

type ModFile struct {
	Sources map[string]SourceConfig `toml:"sources"`
}

type ResolvedModuleSource struct {
	Provider string
	Ref      string
	Version  string
}

var coreModuleDefaults = map[string]string{
	"icons": "base16-icons-linux",
}

func LoadModFile(path string) (*ModFile, error) {
	out := &ModFile{
		Sources: map[string]SourceConfig{},
	}
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return out, nil
	}
	if _, err := toml.DecodeFile(path, out); err != nil {
		return nil, fmt.Errorf("failed to parse %s: %w", path, err)
	}
	if out.Sources == nil {
		out.Sources = map[string]SourceConfig{}
	}
	return out, nil
}

func (m *ModFile) ResolveModuleSource(moduleName, explicitFrom, modulesBaseDir string, sumFile *SumFile) (ResolvedModuleSource, error) {
	spec := strings.TrimSpace(explicitFrom)
	if spec == "" {
		if coreRef, ok := coreModuleDefaults[moduleName]; ok {
			spec = "core:" + coreRef
		}
	}
	if spec == "" {
		spec = "local:" + moduleName
	}

	parts := strings.SplitN(spec, ":", 2)
	if len(parts) != 2 || strings.TrimSpace(parts[0]) == "" || strings.TrimSpace(parts[1]) == "" {
		return ResolvedModuleSource{}, fmt.Errorf("invalid module source %q (expected <source-or-provider>:<path>[@version])", spec)
	}
	left := strings.TrimSpace(parts[0])
	right := strings.TrimSpace(parts[1])

	// Built-in providers (no alias entry required).
	if left == "local" || left == "core" || left == "github" || left == "registry" || left == "http" || left == "https" {
		ref, version := splitRefAndVersion(right)
		resolved, err := applyVersionLock(moduleName, left, ref, version, sumFile)
		if err != nil {
			return ResolvedModuleSource{}, err
		}
		return resolved, validateNonVersionedProvider(resolved)
	}

	// Alias source from workspaced.mod.toml
	src, ok := m.Sources[left]
	if !ok {
		return ResolvedModuleSource{}, fmt.Errorf("unknown source alias %q for module %q", left, moduleName)
	}
	if err := validateSourceLock(left, src, sumFile); err != nil {
		return ResolvedModuleSource{}, err
	}
	provider := strings.TrimSpace(src.Provider)
	if provider == "" {
		provider = left
	}

	switch provider {
	case "local":
		base := strings.TrimSpace(src.Path)
		if base == "" {
			base = modulesBaseDir
		} else if !filepath.IsAbs(base) {
			repoRoot := filepath.Dir(modulesBaseDir)
			base = filepath.Join(repoRoot, base)
		}
		ref, version := splitRefAndVersion(right)
		resolved, err := applyVersionLock(moduleName, "local", filepath.Join(base, ref), version, sumFile)
		if err != nil {
			return ResolvedModuleSource{}, err
		}
		return resolved, validateNonVersionedProvider(resolved)
	case "github":
		repo := strings.Trim(strings.TrimSpace(src.Repo), "/")
		if repo == "" {
			return ResolvedModuleSource{}, fmt.Errorf("source alias %q (github) requires repo", left)
		}
		ref, version := splitRefAndVersion(right)
		path := strings.Trim(strings.TrimSpace(ref), "/")
		fullRef := repo
		if path != "" {
			fullRef = repo + "/" + path
		}
		return applyVersionLock(moduleName, "github", fullRef, version, sumFile)
	default:
		ref, version := splitRefAndVersion(right)
		return applyVersionLock(moduleName, provider, ref, version, sumFile)
	}
}

func splitRefAndVersion(input string) (string, string) {
	in := strings.TrimSpace(input)
	idx := strings.LastIndex(in, "@")
	if idx <= 0 || idx == len(in)-1 {
		return in, ""
	}
	return strings.TrimSpace(in[:idx]), strings.TrimSpace(in[idx+1:])
}

func applyVersionLock(moduleName, provider, ref, version string, sumFile *SumFile) (ResolvedModuleSource, error) {
	resolved := ResolvedModuleSource{
		Provider: strings.TrimSpace(provider),
		Ref:      strings.TrimSpace(ref),
		Version:  strings.TrimSpace(version),
	}
	if sumFile == nil {
		return resolved, nil
	}
	lock, ok := sumFile.Modules[moduleName]
	if !ok {
		return resolved, nil
	}
	expectedSource := resolved.Provider + ":" + resolved.Ref
	if lock.Source != expectedSource {
		return ResolvedModuleSource{}, fmt.Errorf(
			"module %q lock mismatch: source=%q but resolved=%q",
			moduleName, lock.Source, expectedSource,
		)
	}
	if resolved.Version == "" && lock.Version != "" {
		resolved.Version = lock.Version
		return resolved, nil
	}
	if resolved.Version != "" && lock.Version != "" && resolved.Version != lock.Version {
		return ResolvedModuleSource{}, fmt.Errorf(
			"module %q lock mismatch: version=%q but resolved=%q",
			moduleName, lock.Version, resolved.Version,
		)
	}
	return resolved, nil
}

func validateNonVersionedProvider(source ResolvedModuleSource) error {
	if source.Version == "" {
		return nil
	}
	if source.Provider == "local" || source.Provider == "core" {
		return fmt.Errorf("provider %q does not support version pins", source.Provider)
	}
	return nil
}

func validateSourceLock(alias string, src SourceConfig, sumFile *SumFile) error {
	if sumFile == nil {
		return nil
	}
	lock, ok := sumFile.Sources[alias]
	if !ok {
		return nil
	}
	provider := strings.TrimSpace(src.Provider)
	if provider == "" {
		provider = alias
	}
	if strings.TrimSpace(lock.Provider) != provider ||
		strings.TrimSpace(lock.Path) != strings.TrimSpace(src.Path) ||
		strings.TrimSpace(lock.Repo) != strings.TrimSpace(src.Repo) ||
		strings.TrimSpace(lock.URL) != strings.TrimSpace(src.URL) {
		return fmt.Errorf("source %q lock mismatch: run `workspaced mod lock`", alias)
	}
	return nil
}
