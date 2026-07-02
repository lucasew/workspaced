package modfile

import (
	"sort"
	"strings"
)

func BuildRenovateDependencies(sum *SumFile) []RenovateDependency {
	if sum == nil {
		return nil
	}
	return BuildRenovateDependenciesFromLocks(sum.SourceLocks(), sum.ToolLocks())
}

// enrichToolDependency ensures basic fields on a renovate dep entry for a tool.
func enrichToolDependency(dep RenovateDependency) RenovateDependency {
	dep.Kind = "tool"
	dep.Ref = strings.TrimSpace(dep.Ref)
	if dep.Ref == "" {
		return dep
	}
	if dep.CurrentValue == "" {
		// version info should be in CurrentValue by caller
	}
	return dep
}

func BuildRenovateDependenciesFromLocks(sources map[string]LockedSource, tools map[string]LockedTool) []RenovateDependency {
	deps := make([]RenovateDependency, 0, len(tools)+len(sources))
	for _, src := range sources {
		dep := RenovateDependency{
			Kind:         "source",
			Ref:          strings.TrimSpace(src.Ref),
			CurrentValue: strings.TrimSpace(src.Ref),
		}
		if p, ok := getSourceProvider(src.Provider); ok {
			p.EnrichRenovateDependency(&dep, src)
		}
		deps = append(deps, dep)
	}
	for _, tool := range tools {
		ref := strings.TrimSpace(tool.Ref)
		version := strings.TrimSpace(tool.Version)
		if ref == "" || version == "" {
			continue
		}
		dep := RenovateDependency{
			Kind:        "tool",
			Ref:         ref,
			DepName:     strings.TrimSpace(tool.DepName),
			Datasource:  strings.TrimSpace(tool.Datasource),
			PackageName: strings.TrimSpace(tool.PackageName),
			Versioning:  strings.TrimSpace(tool.Versioning),
		}
		if dep.CurrentValue == "" {
			dep.CurrentValue = version
		}
		dep = enrichToolDependency(dep)
		deps = append(deps, dep)
	}

	sort.Slice(deps, func(i, j int) bool {
		if deps[i].Datasource != deps[j].Datasource {
			return deps[i].Datasource < deps[j].Datasource
		}
		if deps[i].DepName != deps[j].DepName {
			return deps[i].DepName < deps[j].DepName
		}
		return deps[i].CurrentValue < deps[j].CurrentValue
	})

	return deps
}

func MergeRenovateDependencies(existing, generated []RenovateDependency) []RenovateDependency {
	out := make([]RenovateDependency, 0, len(existing)+len(generated))
	indexByKey := map[string]int{}

	for _, dep := range existing {
		key := dependencyMergeKey(dep)
		if key == "" {
			continue
		}
		if idx, ok := indexByKey[key]; ok {
			out[idx] = dep
			continue
		}
		indexByKey[key] = len(out)
		out = append(out, dep)
	}

	for _, dep := range generated {
		key := dependencyMergeKey(dep)
		if key == "" {
			continue
		}
		if idx, ok := indexByKey[key]; ok {
			out[idx] = dep
			continue
		}
		indexByKey[key] = len(out)
		out = append(out, dep)
	}

	sort.Slice(out, func(i, j int) bool {
		if out[i].Kind != out[j].Kind {
			return out[i].Kind < out[j].Kind
		}
		if out[i].Ref != out[j].Ref {
			return out[i].Ref < out[j].Ref
		}
		if out[i].Datasource != out[j].Datasource {
			return out[i].Datasource < out[j].Datasource
		}
		return out[i].DepName < out[j].DepName
	})

	return out
}

func dependencyMergeKey(dep RenovateDependency) string {
	kind := strings.TrimSpace(dep.Kind)
	ref := strings.TrimSpace(dep.Ref)
	if kind != "" && ref != "" {
		return kind + ":" + ref
	}
	ds := strings.TrimSpace(dep.Datasource)
	dn := strings.TrimSpace(dep.DepName)
	if ds != "" && dn != "" {
		return ds + ":" + dn
	}
	return ""
}
