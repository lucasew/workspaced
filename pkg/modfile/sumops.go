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
	if s.Tools == nil {
		s.Tools = map[string]LockedTool{}
	}

	current, ok := s.Tools[name]
	changed := !ok || current.Ref != lock.Ref || current.Version != lock.Version
	if changed {
		s.Tools[name] = lock
	}
	if !hasToolDependency(s.Dependencies, name, lock.Ref, lock.Version) {
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
	if s.Sources == nil {
		s.Sources = map[string]LockedSource{}
	}

	current, ok := s.Sources[name]
	changed := !ok ||
		current.Provider != lock.Provider ||
		current.Path != lock.Path ||
		current.Repo != lock.Repo ||
		current.Ref != lock.Ref ||
		current.URL != lock.URL ||
		current.Hash != lock.Hash
	if changed {
		s.Sources[name] = lock
	}
	if !hasSourceDependency(s.Dependencies, name, lock) {
		changed = true
	}
	return changed
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
