package modfile

import (
	"context"
	"strings"
)

func updateSumFile(ctx context.Context, path string, mutate func(sum *SumFile) (bool, error)) (bool, error) {
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
	if err := writeSumFile(ctx, path, sum); err != nil {
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
	if lock.Ref == "" || lock.Version == "" {
		return false
	}
	if s.toolLocks == nil {
		s.toolLocks = map[string]LockedTool{}
	}
	// update runtime alias map
	prev, had := s.toolLocks[name]
	s.toolLocks[name] = lock
	if !had || prev.Version != lock.Version || prev.Ref != lock.Ref {
		// will sync dep below
	}

	// Existence and update in deps list keyed by kind + ref.
	found := false
	changed := false
	for i := range s.Dependencies {
		d := &s.Dependencies[i]
		if d.Kind != "tool" || strings.TrimSpace(d.Ref) != lock.Ref {
			continue
		}
		found = true
		if d.CurrentValue != lock.Version {
			d.CurrentValue = lock.Version
			changed = true
		}
		if lock.DepName != "" && d.DepName != lock.DepName {
			d.DepName = lock.DepName
			changed = true
		}
		if lock.Datasource != "" && d.Datasource != lock.Datasource {
			d.Datasource = lock.Datasource
			changed = true
		}
		if lock.PackageName != "" && d.PackageName != lock.PackageName {
			d.PackageName = lock.PackageName
			changed = true
		}
		if lock.Versioning != "" && d.Versioning != lock.Versioning {
			d.Versioning = lock.Versioning
			changed = true
		}
		break
	}
	if !found {
		s.Dependencies = append(s.Dependencies, RenovateDependency{
			Kind:         "tool",
			Ref:          lock.Ref,
			CurrentValue: lock.Version,
			DepName:      strings.TrimSpace(lock.DepName),
			Datasource:   strings.TrimSpace(lock.Datasource),
			PackageName:  strings.TrimSpace(lock.PackageName),
			Versioning:   strings.TrimSpace(lock.Versioning),
		})
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
	lock.Ref = strings.TrimSpace(lock.Ref)
	lock.Hash = strings.TrimSpace(lock.Hash)
	if lock.Provider == "" || lock.Hash == "" || lock.Ref == "" {
		return false
	}
	if s.sourceLocks == nil {
		s.sourceLocks = map[string]LockedSource{}
	}
	s.sourceLocks[name] = lock

	dep := RenovateDependency{
		Kind:         "source",
		Ref:          lock.Ref,
		CurrentValue: lock.Ref,
		// Intentionally omit CurrentDigest: renovate's custom manager cannot
		// handle digest updates for source pins (commit SHAs live in currentValue).
	}
	if p, ok := getSourceProvider(lock.Provider); ok {
		p.EnrichRenovateDependency(&dep, lock)
	}

	// Keyed by kind + ref (the provider-filled stable ref) in the deps list.
	found := false
	changed := false
	for i := range s.Dependencies {
		d := &s.Dependencies[i]
		if d.Kind != "source" || strings.TrimSpace(d.Ref) != dep.Ref {
			continue
		}
		found = true
		if d.CurrentValue != dep.CurrentValue {
			d.CurrentValue = dep.CurrentValue
			changed = true
		}
		// Drop any previously-written digest; renovate cannot handle it.
		if d.CurrentDigest != "" {
			d.CurrentDigest = ""
			changed = true
		}
		if dep.DepName != "" && d.DepName != dep.DepName {
			d.DepName = dep.DepName
			changed = true
		}
		if dep.Datasource != "" && d.Datasource != dep.Datasource {
			d.Datasource = dep.Datasource
			changed = true
		}
		break
	}
	if !found {
		s.Dependencies = append(s.Dependencies, dep)
		changed = true
	}
	return changed
}
