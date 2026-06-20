package modfile

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"
)

type LockedSource struct {
	Provider string `json:"provider"`
	Path     string `json:"path"`
	Repo     string `json:"repo"`
	Ref      string `json:"ref,omitempty"`
	URL      string `json:"url"`
	Hash     string `json:"hash"`
}

type LockedTool struct {
	Ref     string `json:"ref"`
	Version string `json:"version"`

	// Renovate reference fields (populated by default via EnrichLockfile
	// on the live Tool when locking). These are the "data apart from
	// toolName and version" that instruct renovate how to fetch updates.
	DepName     string `json:"depName,omitempty"`
	Datasource  string `json:"datasource,omitempty"`
	PackageName string `json:"packageName,omitempty"`
	Versioning  string `json:"versioning,omitempty"`
}
type RenovateDependency struct {
	// === Workspace-specific / custom fields ===

	// Kind is your own category or classification for the dependency.
	// Examples: "library", "tool", "internal", "cli"
	Kind string `json:"kind,omitempty"`

	// Ref is your internal reference or identifier for this dependency.
	// Can be a repository path, module path, or any custom reference.
	Ref string `json:"ref,omitempty"`

	// === Core identification ===

	// DepName is the human-readable name of the package.
	// This is what appears in Renovate PR titles, commit messages,
	// and the Dependency Dashboard.
	DepName string `json:"depName"`

	// PackageName is the exact name used for registry lookup.
	// Use this when it differs from DepName (e.g. scoped packages).
	// Falls back to DepName if not set.
	PackageName string `json:"packageName,omitempty"`

	// === Current version information ===

	// CurrentValue is the exact version (or version range) string
	// currently present in your JSON file.
	// This field is required.
	CurrentValue string `json:"currentValue"`

	// CurrentDigest is the current digest/hash.
	// Mainly used for Docker/OCI images.
	CurrentDigest string `json:"currentDigest,omitempty"`

	// CurrentVersion is the resolved/pinned version (if different
	// from CurrentValue). Optional.
	CurrentVersion string `json:"currentVersion,omitempty"`

	// === Lookup & source configuration ===

	// Datasource tells Renovate where to fetch new versions from.
	// Required. Common values: "npm", "go", "docker", "github-tags",
	// "maven", "pypi", etc.
	Datasource string `json:"datasource"`

	// Versioning defines the version comparison strategy.
	// Common values: "semver", "docker", "loose", "pep440", "maven"
	Versioning string `json:"versioning,omitempty"`

	// ExtractVersion is a regex used to extract a clean version
	// from messy tags (e.g. "v1.2.3-alpine" or "release-2024.01").
	// Use named capture group: (?<version>...)
	ExtractVersion string `json:"extractVersion,omitempty"`

	// RegistryUrls contains custom or private registry URLs.
	RegistryUrls []string `json:"registryUrls,omitempty"`

	// === Optional context / metadata ===

	// DepType indicates the type/category of the dependency.
	// Useful for applying different packageRules.
	// Examples: "dependencies", "devDependencies", "tools"
	DepType string `json:"depType,omitempty"`

	// SourceUrl is the URL to the project's homepage or repository.
	SourceUrl string `json:"sourceUrl,omitempty"`

	// Manager is an optional hint for which Renovate manager should handle this.
	Manager string `json:"manager,omitempty"`

	// SkipReason tells Renovate to skip this dependency and why.
	// Common values: "disabled", "ignored", "not-supported"
	SkipReason string `json:"skipReason,omitempty"`
}
type SumFile struct {
	Dependencies []RenovateDependency `json:"dependencies,omitempty"`

	// runtime alias -> lock (not serialized to sumFileDisk; aliases are
	// not part of the clean renovate dep entries)
	sourceLocks map[string]LockedSource
	toolLocks   map[string]LockedTool
}

type sumFileDisk struct {
	Dependencies []RenovateDependency `json:"dependencies,omitempty"`
}

func LoadSumFile(path string) (*SumFile, error) {
	out := &SumFile{}
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return out, nil
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read %s: %w", path, err)
	}
	var disk sumFileDisk
	if err := json.Unmarshal(data, &disk); err != nil {
		return nil, fmt.Errorf("failed to parse %s: %w", path, err)
	}

	out.Dependencies = disk.Dependencies
	if err := normalizeDependencies(out.Dependencies); err != nil {
		return nil, err
	}
	return out, nil
}

func normalizeDependencies(deps []RenovateDependency) error {
	for i, dep := range deps {
		dep.Kind = strings.TrimSpace(dep.Kind)
		dep.Ref = strings.TrimSpace(dep.Ref)
		dep.DepName = strings.TrimSpace(dep.DepName)
		dep.CurrentValue = strings.TrimSpace(dep.CurrentValue)
		dep.Datasource = strings.TrimSpace(dep.Datasource)
		dep.PackageName = strings.TrimSpace(dep.PackageName)
		dep.Versioning = strings.TrimSpace(dep.Versioning)
		deps[i] = dep
	}

	// Store dependencies sorted by Ref for deterministic lockfiles.
	sort.Slice(deps, func(i, j int) bool {
		return strings.TrimSpace(deps[i].Ref) < strings.TrimSpace(deps[j].Ref)
	})
	return nil
}

func rebuildSourceLocksFromDependencies(sum *SumFile) map[string]LockedSource {
	out := map[string]LockedSource{}
	if sum == nil {
		return out
	}
	for _, dep := range sum.Dependencies {
		if dep.Kind != "source" {
			continue
		}
		key := strings.TrimSpace(dep.Ref)
		if key == "" {
			key = strings.TrimSpace(dep.DepName)
		}
		if key == "" {
			continue
		}
		// Best effort from available clean fields. Hash may be in CurrentDigest.
		hash := strings.TrimSpace(dep.CurrentDigest)
		out[key] = LockedSource{
			Ref:  key,
			Hash: hash,
		}
	}
	return out
}

func rebuildToolLocksFromDependencies(sum *SumFile) map[string]LockedTool {
	out := map[string]LockedTool{}
	if sum == nil {
		return out
	}
	for _, dep := range sum.Dependencies {
		if dep.Kind != "tool" {
			continue
		}
		key := strings.TrimSpace(dep.Ref)
		if key == "" {
			key = strings.TrimSpace(dep.DepName)
		}
		if key == "" {
			continue
		}
		version := strings.TrimSpace(dep.CurrentValue)
		if version == "" {
			version = strings.TrimSpace(dep.CurrentVersion)
		}
		if version == "" {
			continue
		}
		out[key] = LockedTool{
			Ref:         key,
			Version:     version,
			DepName:     strings.TrimSpace(dep.DepName),
			Datasource:  strings.TrimSpace(dep.Datasource),
			PackageName: strings.TrimSpace(dep.PackageName),
			Versioning:  strings.TrimSpace(dep.Versioning),
		}
	}
	return out
}

func (s *SumFile) SourceLocks() map[string]LockedSource {
	if s != nil && len(s.sourceLocks) > 0 {
		cp := make(map[string]LockedSource, len(s.sourceLocks))
		for k, v := range s.sourceLocks {
			cp[k] = v
		}
		return cp
	}
	return rebuildSourceLocksFromDependencies(s)
}

func (s *SumFile) ToolLocks() map[string]LockedTool {
	if s != nil && len(s.toolLocks) > 0 {
		cp := make(map[string]LockedTool, len(s.toolLocks))
		for k, v := range s.toolLocks {
			cp[k] = v
		}
		return cp
	}
	return rebuildToolLocksFromDependencies(s)
}

func (s *SumFile) FindSource(name string) (LockedSource, bool) {
	name = strings.TrimSpace(name)
	if name == "" {
		return LockedSource{}, false
	}
	locks := s.SourceLocks()
	if lock, ok := locks[name]; ok {
		return lock, true
	}
	// Fallback: scan deps using kind + ref (or depName) as identity.
	for _, dep := range s.Dependencies {
		if dep.Kind != "source" {
			continue
		}
		if dep.Ref == name || dep.DepName == name {
			return LockedSource{Ref: dep.Ref, Hash: dep.CurrentDigest}, true
		}
	}
	return LockedSource{}, false
}

func (s *SumFile) FindTool(name string) (LockedTool, bool) {
	name = strings.TrimSpace(name)
	if name == "" {
		return LockedTool{}, false
	}
	locks := s.ToolLocks()
	if lock, ok := locks[name]; ok {
		return lock, true
	}
	// Fallback scan using kind + ref as key per updated model.
	for _, dep := range s.Dependencies {
		if dep.Kind != "tool" {
			continue
		}
		if dep.Ref == name || dep.DepName == name {
			ver := dep.CurrentValue
			if ver == "" {
				ver = dep.CurrentVersion
			}
			return LockedTool{
				Ref:         dep.Ref,
				Version:     ver,
				DepName:     dep.DepName,
				Datasource:  dep.Datasource,
				PackageName: dep.PackageName,
				Versioning:  dep.Versioning,
			}, true
		}
	}
	return LockedTool{}, false
}
