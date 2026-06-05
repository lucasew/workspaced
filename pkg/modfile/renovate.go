package modfile

import (
	"net/url"
	"sort"
	"strings"

	parsespec "workspaced/pkg/parse/spec"
)

func BuildRenovateDependencies(sum *SumFile) []RenovateDependency {
	if sum == nil {
		return nil
	}
	return BuildRenovateDependenciesFromLocks(sum.SourceLocks(), sum.ToolLocks())
}

// enrichToolDependency is now minimal: it only fills basic tool lock state
// (kind, name, ref, version, currentValue, provider). Most logic for the
// renovate reference (depName/datasource etc.) lives in the Tool
// implementations (via EnrichLockfile). Callers that have a live Tool
// (e.g. lazy locking) obtain the data by calling EnrichLockfile on a
// (temp or real) RenovateDependency; this func is mainly for normalizing
// the common fields.
func enrichToolDependency(dep RenovateDependency) RenovateDependency {
	dep.Kind = "tool"
	dep.Name = strings.TrimSpace(dep.Name)
	dep.Ref = strings.TrimSpace(dep.Ref)
	dep.Version = strings.TrimSpace(dep.Version)
	dep.Provider = strings.TrimSpace(dep.Provider)

	if dep.Ref == "" || dep.Version == "" {
		return dep
	}

	dep.CurrentValue = dep.Version

	if dep.Provider == "" {
		if spec, err := parsespec.Parse(dep.Ref); err == nil {
			dep.Provider = spec.Provider
		}
	}

	return dep
}

func BuildRenovateDependenciesFromLocks(sources map[string]LockedSource, tools map[string]LockedTool) []RenovateDependency {
	deps := make([]RenovateDependency, 0, len(tools)+len(sources))
	for alias, src := range sources {
		dep := RenovateDependency{
			Kind:     "source",
			Name:     strings.TrimSpace(alias),
			Provider: strings.TrimSpace(src.Provider),
			Path:     strings.TrimSpace(src.Path),
			Repo:     strings.TrimSpace(src.Repo),
			Ref:      strings.TrimSpace(src.Ref),
			URL:      strings.TrimSpace(src.URL),
			Hash:     strings.TrimSpace(src.Hash),
		}
		// Always persist lock state for sources. Renovate fields are optional.
		deps = append(deps, dep)

		switch strings.TrimSpace(src.Provider) {
		case "github":
			depName := strings.TrimSpace(src.Repo)
			if depName == "" {
				continue
			}
			currentValue := strings.TrimSpace(src.Ref)
			if strings.EqualFold(currentValue, "HEAD") {
				currentValue = ""
			}
			if currentValue == "" {
				currentValue = refFromTarballURL(src.URL)
			}
			if currentValue == "" {
				continue
			}
			dep.DepName = depName
			dep.CurrentValue = currentValue
			dep.Datasource = "github-tags"
			deps[len(deps)-1] = dep
		}
	}
	for name, tool := range tools {
		ref := strings.TrimSpace(tool.Ref)
		version := strings.TrimSpace(tool.Version)
		if ref == "" || version == "" {
			continue
		}

		// The renovate reference (depName, datasource, ...) is populated
		// by calling EnrichLockfile on the live Tool (at lock time).
		// enrichToolDependency only fills the basic ref/version lock state.
		dep := RenovateDependency{
			Kind:        "tool",
			Name:        strings.TrimSpace(name),
			Ref:         ref,
			Version:     version,
			DepName:     strings.TrimSpace(tool.DepName),
			Datasource:  strings.TrimSpace(tool.Datasource),
			PackageName: strings.TrimSpace(tool.PackageName),
			Versioning:  strings.TrimSpace(tool.Versioning),
		}
		dep = enrichToolDependency(dep)
		// Always persist lock state for tools. Renovate fields come from the Tool.
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
		if out[i].Name != out[j].Name {
			return out[i].Name < out[j].Name
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
	name := strings.TrimSpace(dep.Name)
	if kind != "" && name != "" {
		return kind + ":" + name
	}
	ds := strings.TrimSpace(dep.Datasource)
	dn := strings.TrimSpace(dep.DepName)
	if ds != "" && dn != "" {
		return ds + ":" + dn
	}
	return ""
}

func refFromTarballURL(rawURL string) string {
	rawURL = strings.TrimSpace(rawURL)
	if rawURL == "" {
		return ""
	}
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return ""
	}
	if !strings.EqualFold(parsed.Hostname(), "codeload.github.com") {
		return ""
	}
	parts := strings.Split(strings.Trim(parsed.Path, "/"), "/")
	// codeload.github.com/<owner>/<repo>/tar.gz/<ref>
	if len(parts) != 4 || parts[2] != "tar.gz" {
		return ""
	}
	return strings.TrimSpace(parts[3])
}
