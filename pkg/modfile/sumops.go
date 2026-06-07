package modfile

import (
	"strings"

	parsespec "workspaced/pkg/parse/spec"
)

func updateSumFile(path string, mutate func(sum *SumFile) (bool, error)) (bool, error) {
	sum, err := LoadSumFile(path)
	if err != nil {
		return false, err
	}
	if mutate == nil {
		return false, nil
	}
	changed, err := mutate(sum)
	if err != nil {
		return false, err
	}
	if !changed {
		return false, nil
	}
	if err := writeSumFile(path, sum); err != nil {
		return false, err
	}
	return true, nil
}

func (s *SumFile) Tool(name string) (LockedTool, bool) {
	if s == nil {
		return LockedTool{}, false
	}
	return s.FindTool(name)
}

func (s *SumFile) Source(name string) (LockedSource, bool) {
	if s == nil {
		return LockedSource{}, false
	}
	return s.FindSource(name)
}

func (s *SumFile) EnsureTool(name string, lock LockedTool) bool {
	return s.UpsertTool(name, lock)
}

func (s *SumFile) EnsureSource(name string, lock LockedSource) bool {
	return s.UpsertSource(name, lock)
}

func (s *SumFile) UpsertTool(name string, lock LockedTool) bool {
	if s == nil {
		return false
	}
	name = strings.TrimSpace(name)
	lock.Ref = strings.TrimSpace(lock.Ref)
	lock.Version = strings.TrimSpace(lock.Version)
	if name == "" || lock.Ref == "" || lock.Version == "" {
		return false
	}
	current, ok := s.FindTool(name)
	changed := !ok || current.Ref != lock.Ref || current.Version != lock.Version
	if currentValue := dependencyCurrentValue(s.Dependencies, "tool", name); currentValue != "" && currentValue != lock.Version {
		changed = true
	}
	// Only consider renovate fields for change detection if the incoming lock
	// provides them (allows passing minimal LockedTool for idempotency tests
	// or legacy paths; live paths from Tool always provide when available).
	if lock.DepName != "" && current.DepName != lock.DepName {
		changed = true
	}
	if lock.Datasource != "" && current.Datasource != lock.Datasource {
		changed = true
	}
	if lock.PackageName != "" && current.PackageName != lock.PackageName {
		changed = true
	}
	if lock.Versioning != "" && current.Versioning != lock.Versioning {
		changed = true
	}
	if !hasToolDependency(s.Dependencies, name, lock.Ref, lock.Version) || changed {
		s.Dependencies = upsertToolDependency(s.Dependencies, name, lock)
		changed = true
	}
	return changed
}

func (s *SumFile) UpsertSource(name string, lock LockedSource) bool {
	if s == nil {
		return false
	}
	name = strings.TrimSpace(name)
	lock.Provider = strings.TrimSpace(lock.Provider)
	lock.Path = strings.TrimSpace(lock.Path)
	lock.Repo = strings.TrimSpace(lock.Repo)
	lock.Ref = strings.TrimSpace(lock.Ref)
	lock.URL = strings.TrimSpace(lock.URL)
	lock.Hash = strings.TrimSpace(lock.Hash)
	if name == "" || lock.Provider == "" || lock.Hash == "" {
		return false
	}
	current, ok := s.FindSource(name)
	changed := !ok ||
		current.Provider != lock.Provider ||
		current.Path != lock.Path ||
		current.Repo != lock.Repo ||
		current.Ref != lock.Ref ||
		current.URL != lock.URL ||
		current.Hash != lock.Hash
	if !hasSourceDependency(s.Dependencies, name, lock) || changed {
		s.Dependencies = upsertSourceDependency(s.Dependencies, name, lock)
		changed = true
	}
	return changed
}

func dependencyCurrentValue(deps []RenovateDependency, kind, name string) string {
	for _, dep := range deps {
		if strings.TrimSpace(dep.Kind) == kind && strings.TrimSpace(dep.Name) == name {
			return strings.TrimSpace(dep.CurrentValue)
		}
	}
	return ""
}

func upsertToolDependency(deps []RenovateDependency, name string, lock LockedTool) []RenovateDependency {
	for i, dep := range deps {
		if strings.TrimSpace(dep.Kind) == "tool" && strings.TrimSpace(dep.Name) == name {
			dep.Kind = "tool"
			dep.Name = name
			dep.Ref = lock.Ref
			dep.Version = lock.Version
			dep.CurrentValue = lock.Version
			// If the caller (live lock from Tool) provided renovate reference data,
			// use it (this is the preferred path, data comes from EnrichLockfile
			// on the live Tool). Otherwise fall back to slim enrich.
			if lock.DepName != "" || lock.Datasource != "" {
				dep.DepName = lock.DepName
				dep.Datasource = lock.Datasource
				dep.PackageName = lock.PackageName
				dep.Versioning = lock.Versioning
				dep.Provider = strings.TrimSpace(dep.Provider)
				if dep.Provider == "" {
					// best effort from ref
					if spec, err := parsespec.Parse(lock.Ref); err == nil {
						dep.Provider = spec.Provider
					}
				}
			} else {
				dep = enrichToolDependency(dep)
			}
			deps[i] = dep
			return deps
		}
	}
	dep := RenovateDependency{
		Kind:         "tool",
		Name:         name,
		Ref:          lock.Ref,
		Version:      lock.Version,
		CurrentValue: lock.Version,
	}
	if lock.DepName != "" || lock.Datasource != "" {
		dep.DepName = lock.DepName
		dep.Datasource = lock.Datasource
		dep.PackageName = lock.PackageName
		dep.Versioning = lock.Versioning
		if spec, err := parsespec.Parse(lock.Ref); err == nil {
			dep.Provider = spec.Provider
		}
	} else {
		dep = enrichToolDependency(dep)
	}
	return append(deps, dep)
}

func upsertSourceDependency(deps []RenovateDependency, name string, lock LockedSource) []RenovateDependency {
	for i, dep := range deps {
		if strings.TrimSpace(dep.Kind) == "source" && strings.TrimSpace(dep.Name) == name {
			dep.Kind = "source"
			dep.Name = name
			dep.Provider = lock.Provider
			dep.Path = lock.Path
			dep.Repo = lock.Repo
			dep.Ref = lock.Ref
			dep.URL = lock.URL
			dep.Hash = lock.Hash
			dep.CurrentValue = lock.Ref
			deps[i] = dep
			return deps
		}
	}
	return append(deps, RenovateDependency{
		Kind:     "source",
		Name:     name,
		Provider: lock.Provider,
		Path:     lock.Path,
		Repo:     lock.Repo,
		Ref:      lock.Ref,
		URL:      lock.URL,
		Hash:     lock.Hash,
	})
}

func hasToolDependency(deps []RenovateDependency, name, ref, version string) bool {
	for _, dep := range deps {
		if strings.TrimSpace(dep.Kind) != "tool" {
			continue
		}
		if strings.TrimSpace(dep.Name) != name {
			continue
		}
		if strings.TrimSpace(dep.Ref) != ref {
			continue
		}
		effectiveVersion := strings.TrimSpace(dep.Version)
		if effectiveVersion == "" {
			effectiveVersion = strings.TrimSpace(dep.CurrentValue)
		}
		if effectiveVersion == version {
			return true
		}
	}
	return false
}

func hasSourceDependency(deps []RenovateDependency, name string, lock LockedSource) bool {
	for _, dep := range deps {
		if strings.TrimSpace(dep.Kind) != "source" {
			continue
		}
		if strings.TrimSpace(dep.Name) != name {
			continue
		}
		effectiveRef := strings.TrimSpace(dep.Ref)
		if effectiveRef == "" {
			effectiveRef = strings.TrimSpace(dep.CurrentValue)
		}
		if strings.TrimSpace(dep.Provider) != lock.Provider {
			continue
		}
		if strings.TrimSpace(dep.Path) != lock.Path {
			continue
		}
		if strings.TrimSpace(dep.Repo) != lock.Repo {
			continue
		}
		if effectiveRef != lock.Ref {
			continue
		}
		if strings.TrimSpace(dep.URL) != lock.URL {
			continue
		}
		if strings.TrimSpace(dep.Hash) != lock.Hash {
			continue
		}
		return true
	}
	return false
}
