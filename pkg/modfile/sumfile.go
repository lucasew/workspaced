package modfile

import (
	"encoding/json"
	"fmt"
	"os"
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
}

type RenovateDependency struct {
	Kind string `json:"kind,omitempty"`
	Name string `json:"name,omitempty"`

	Provider string `json:"provider,omitempty"`
	Ref      string `json:"ref,omitempty"`
	Version  string `json:"version,omitempty"`
	Path     string `json:"path,omitempty"`
	Repo     string `json:"repo,omitempty"`
	URL      string `json:"url,omitempty"`
	Hash     string `json:"hash,omitempty"`

	DepName      string `json:"depName,omitempty"`
	CurrentValue string `json:"currentValue,omitempty"`
	Datasource   string `json:"datasource,omitempty"`
	PackageName  string `json:"packageName,omitempty"`
	Versioning   string `json:"versioning,omitempty"`
}

type SumFile struct {
	Dependencies []RenovateDependency `json:"dependencies,omitempty"`
}

type sumFileDisk struct {
	Dependencies []RenovateDependency    `json:"dependencies,omitempty"`
	Sources      map[string]LockedSource `json:"sources,omitempty"` // backward-compat read only
	Tools        map[string]LockedTool   `json:"tools,omitempty"`   // backward-compat read only
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

	// Preferred format: dependencies-only.
	if len(disk.Dependencies) > 0 {
		out.Dependencies = disk.Dependencies
		if err := normalizeDependencies(out.Dependencies); err != nil {
			return nil, err
		}
		return out, nil
	}

	// Backward compatibility for old lockfiles.
	if err := normalizeSources(disk.Sources); err != nil {
		return nil, err
	}
	if err := normalizeTools(disk.Tools); err != nil {
		return nil, err
	}
	out.Dependencies = BuildRenovateDependenciesFromLocks(disk.Sources, disk.Tools)
	return out, nil
}

func normalizeDependencies(deps []RenovateDependency) error {
	for i, dep := range deps {
		dep.Kind = strings.TrimSpace(dep.Kind)
		dep.Name = strings.TrimSpace(dep.Name)
		dep.Provider = strings.TrimSpace(dep.Provider)
		dep.Ref = strings.TrimSpace(dep.Ref)
		dep.Version = strings.TrimSpace(dep.Version)
		dep.Path = strings.TrimSpace(dep.Path)
		dep.Repo = strings.TrimSpace(dep.Repo)
		dep.URL = strings.TrimSpace(dep.URL)
		dep.Hash = strings.TrimSpace(dep.Hash)
		dep.DepName = strings.TrimSpace(dep.DepName)
		dep.CurrentValue = strings.TrimSpace(dep.CurrentValue)
		dep.Datasource = strings.TrimSpace(dep.Datasource)
		dep.PackageName = strings.TrimSpace(dep.PackageName)
		dep.Versioning = strings.TrimSpace(dep.Versioning)
		switch dep.Kind {
		case "source":
			if dep.Ref == "" {
				dep.Ref = dep.CurrentValue
			}
		case "tool":
			if dep.Version == "" {
				dep.Version = dep.CurrentValue
			}
		}
		deps[i] = dep
	}
	return nil
}

func normalizeSources(sources map[string]LockedSource) error {
	for name, lock := range sources {
		lock.Provider = strings.TrimSpace(lock.Provider)
		lock.Path = strings.TrimSpace(lock.Path)
		lock.Repo = strings.TrimSpace(lock.Repo)
		lock.Ref = strings.TrimSpace(lock.Ref)
		lock.URL = strings.TrimSpace(lock.URL)
		lock.Hash = strings.TrimSpace(lock.Hash)
		if lock.Provider == "" {
			return fmt.Errorf("invalid lock entry for source %q: provider is required", name)
		}
		if lock.Hash == "" {
			return fmt.Errorf("invalid lock entry for source %q: hash is required", name)
		}
		sources[name] = lock
	}
	return nil
}

func normalizeTools(tools map[string]LockedTool) error {
	for name, lock := range tools {
		lock.Ref = strings.TrimSpace(lock.Ref)
		lock.Version = strings.TrimSpace(lock.Version)
		if lock.Ref == "" {
			return fmt.Errorf("invalid lock entry for tool %q: ref is required", name)
		}
		if lock.Version == "" {
			return fmt.Errorf("invalid lock entry for tool %q: version is required", name)
		}
		tools[name] = lock
	}
	return nil
}

func rebuildSourceLocksFromDependencies(sum *SumFile) map[string]LockedSource {
	out := map[string]LockedSource{}
	if sum == nil {
		return out
	}
	for _, dep := range sum.Dependencies {
		switch dep.Kind {
		case "source":
			if dep.Name == "" || dep.Provider == "" || dep.Hash == "" {
				continue
			}
			ref := dep.Ref
			if dep.CurrentValue != "" {
				ref = dep.CurrentValue
			}
			out[dep.Name] = LockedSource{
				Provider: dep.Provider,
				Path:     dep.Path,
				Repo:     dep.Repo,
				Ref:      ref,
				URL:      dep.URL,
				Hash:     dep.Hash,
			}
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
		if dep.Kind != "tool" || dep.Name == "" || dep.Ref == "" {
			continue
		}
		version := dep.Version
		if dep.CurrentValue != "" {
			version = dep.CurrentValue
		}
		if version == "" {
			continue
		}
		out[dep.Name] = LockedTool{
			Ref:     dep.Ref,
			Version: version,
		}
	}
	return out
}

func (s *SumFile) SourceLocks() map[string]LockedSource {
	return rebuildSourceLocksFromDependencies(s)
}

func (s *SumFile) ToolLocks() map[string]LockedTool {
	return rebuildToolLocksFromDependencies(s)
}

func (s *SumFile) FindSource(name string) (LockedSource, bool) {
	lock, ok := s.SourceLocks()[strings.TrimSpace(name)]
	return lock, ok
}

func (s *SumFile) FindTool(name string) (LockedTool, bool) {
	lock, ok := s.ToolLocks()[strings.TrimSpace(name)]
	return lock, ok
}
