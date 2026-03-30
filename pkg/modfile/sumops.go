package modfile

import "strings"

func UpdateSumFile(path string, mutate func(sum *SumFile) (bool, error)) (bool, error) {
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
	if err := WriteSumFile(path, sum); err != nil {
		return false, err
	}
	return true, nil
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

func upsertToolDependency(deps []RenovateDependency, name string, lock LockedTool) []RenovateDependency {
	for i, dep := range deps {
		if strings.TrimSpace(dep.Kind) == "tool" && strings.TrimSpace(dep.Name) == name {
			dep.Kind = "tool"
			dep.Name = name
			dep.Ref = lock.Ref
			dep.Version = lock.Version
			if strings.TrimSpace(dep.CurrentValue) == "" || strings.TrimSpace(dep.CurrentValue) == strings.TrimSpace(dep.Version) {
				dep.CurrentValue = lock.Version
			}
			deps[i] = dep
			return deps
		}
	}
	return append(deps, RenovateDependency{
		Kind:    "tool",
		Name:    name,
		Ref:     lock.Ref,
		Version: lock.Version,
	})
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
			if strings.TrimSpace(dep.CurrentValue) == "" || strings.TrimSpace(dep.CurrentValue) == strings.TrimSpace(dep.Ref) {
				dep.CurrentValue = lock.Ref
			}
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
