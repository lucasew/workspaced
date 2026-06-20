package modfile

import "testing"

func TestBuildRenovateDependenciesFromTools(t *testing.T) {
	t.Parallel()

	sources := map[string]LockedSource{
		"icons": {
			Provider: "github",
			Repo:     "PapirusDevelopmentTeam/papirus-icon-theme",
			Ref:      "v2026.03.01",
		},
		"theme": {
			Provider: "github",
			Repo:     "catppuccin/gtk",
			URL:      "https://codeload.github.com/catppuccin/gtk/tar.gz/9aa0d1f",
		},
	}
	tools := map[string]LockedTool{
		"fd":   {Ref: "github:sharkdp/fd", Version: "v10.3.0", DepName: "sharkdp/fd", Datasource: "github-releases"},
		"fzf":  {Ref: "mise:fzf", Version: "0.50.0"},
		"bad1": {Ref: "", Version: "1.0.0"},
		"bad2": {Ref: "github:foo/bar", Version: ""},
	}

	got := BuildRenovateDependenciesFromLocks(sources, tools)
	if len(got) != 4 {
		t.Fatalf("expected 4 dependencies (2 sources + github tool + mise tool), got=%d (%#v)", len(got), got)
	}
	byName := map[string]RenovateDependency{}
	for _, dep := range got {
		byName[dep.DepName] = dep
	}
	sourceDep, ok := byName["PapirusDevelopmentTeam/papirus-icon-theme"]
	if !ok {
		t.Fatalf("missing source dependency for papirus: %#v", got)
	}
	if sourceDep.Datasource != "github-tags" {
		t.Fatalf("datasource mismatch for source dep: got=%q", sourceDep.Datasource)
	}
	if sourceDep.CurrentValue != "v2026.03.01" {
		t.Fatalf("currentValue mismatch for source dep: got=%q", sourceDep.CurrentValue)
	}
	toolDep, ok := byName["sharkdp/fd"]
	if !ok {
		t.Fatalf("missing tool dependency for sharkdp/fd: %#v", got)
	}
	if toolDep.Datasource != "github-releases" {
		t.Fatalf("datasource mismatch for tool dep: got=%q", toolDep.Datasource)
	}
	if toolDep.CurrentValue != "v10.3.0" {
		t.Fatalf("currentValue mismatch for tool dep: got=%q", toolDep.CurrentValue)
	}
	if toolDep.Datasource != "github-releases" {
		t.Fatalf("tool dep missing renovate fields: %#v", toolDep)
	}

	// mise tool produces entry keyed by ref, no extra provider/name.
	var fzfDep *RenovateDependency
	for i := range got {
		if got[i].Ref == "mise:fzf" {
			fzfDep = &got[i]
			break
		}
	}
	if fzfDep == nil {
		t.Fatalf("expected fzf tool dep to be included for lock state")
	}
	if fzfDep.Datasource != "" {
		t.Fatalf("mise tool should not have renovate datasource, got=%s", fzfDep.Datasource)
	}

	themeDep, ok := byName["catppuccin/gtk"]
	if !ok {
		t.Fatalf("missing source dependency for catppuccin/gtk: %#v", got)
	}
	if themeDep.CurrentValue != "9aa0d1f" {
		t.Fatalf("currentValue mismatch for source URL dep: got=%q", themeDep.CurrentValue)
	}
}

func TestBuildRenovateDependenciesSkipsHeadWithoutResolvedURL(t *testing.T) {
	t.Parallel()

	sources := map[string]LockedSource{
		"icons": {
			Provider: "github",
			Repo:     "PapirusDevelopmentTeam/papirus-icon-theme",
			Ref:      "HEAD",
		},
	}

	got := BuildRenovateDependenciesFromLocks(sources, nil)
	// We always persist the lock state entry for the source (renovate fields
	// are optional / best-effort). The "skip" is only for attaching renovate
	// update instructions when we have no usable ref/URL for the datasource.
	if len(got) != 1 {
		t.Fatalf("expected 1 (lock state) dependency, got=%d (%#v)", len(got), got)
	}
	if got[0].Kind != "source" || got[0].DepName != "" {
		t.Fatalf("expected basic source lock state without renovate fields, got=%#v", got[0])
	}
}

func TestMergeRenovateDependenciesPreservesUntouchedEntries(t *testing.T) {
	t.Parallel()

	existing := []RenovateDependency{
		{
			Kind:         "tool",
			Ref:          "github:sharkdp/fd",
			DepName:      "sharkdp/fd",
			CurrentValue: "v10.2.0",
			Datasource:   "github-releases",
		},
		{
			Kind:         "source",
			Ref:          "v2026.02.01",
			DepName:      "PapirusDevelopmentTeam/papirus-icon-theme",
			CurrentValue: "v2026.02.01",
			Datasource:   "github-tags",
		},
	}

	generated := []RenovateDependency{
		{
			Kind:         "tool",
			Ref:          "github:sharkdp/fd",
			DepName:      "sharkdp/fd",
			CurrentValue: "v10.3.0",
			Datasource:   "github-releases",
		},
	}

	got := MergeRenovateDependencies(existing, generated)
	if len(got) != 2 {
		t.Fatalf("expected 2 dependencies after merge, got=%d", len(got))
	}

	byRef := map[string]RenovateDependency{}
	for _, dep := range got {
		k := dep.Ref
		if k == "" {
			k = dep.DepName
		}
		byRef[k] = dep
	}
	if byRef["github:sharkdp/fd"].CurrentValue != "v10.3.0" {
		t.Fatalf("expected fd to be updated, got=%q", byRef["github:sharkdp/fd"].CurrentValue)
	}
	if byRef["v2026.02.01"].CurrentValue != "v2026.02.01" {
		t.Fatalf("expected icons to be preserved, got=%q", byRef["v2026.02.01"].CurrentValue)
	}
}
