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
			// LockHash writes the default branch into Ref; commit pin is in URL.
			Ref: "main",
			URL: "https://codeload.github.com/catppuccin/gtk/tar.gz/9aa0d1fabc1234",
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
	if sourceDep.Datasource != "git-refs" {
		t.Fatalf("datasource mismatch for source dep: got=%q", sourceDep.Datasource)
	}
	if sourceDep.PackageName != "https://github.com/PapirusDevelopmentTeam/papirus-icon-theme" {
		t.Fatalf("packageName mismatch for source dep: got=%q", sourceDep.PackageName)
	}
	// Explicit non-SHA ref is the tracked git ref; no commit pin yet.
	if sourceDep.CurrentValue != "v2026.03.01" {
		t.Fatalf("currentValue mismatch for source dep: got=%q", sourceDep.CurrentValue)
	}
	if sourceDep.CurrentDigest != "" {
		t.Fatalf("source dep without resolved SHA must not set currentDigest, got=%q", sourceDep.CurrentDigest)
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
	if themeDep.Datasource != "git-refs" {
		t.Fatalf("datasource mismatch for theme dep: got=%q", themeDep.Datasource)
	}
	if themeDep.CurrentValue != "main" {
		t.Fatalf("tracking ref should be default branch name, got=%q", themeDep.CurrentValue)
	}
	if themeDep.CurrentDigest != "9aa0d1fabc1234" {
		t.Fatalf("currentDigest mismatch for source URL dep: got=%q", themeDep.CurrentDigest)
	}
	if themeDep.PackageName != "https://github.com/catppuccin/gtk" {
		t.Fatalf("packageName mismatch for theme dep: got=%q", themeDep.PackageName)
	}
}

func TestBuildRenovateDependenciesSkipsHeadWithoutBranch(t *testing.T) {
	t.Parallel()

	// HEAD alone (no resolved branch from LockHash) is not usable with git-refs.
	sources := map[string]LockedSource{
		"icons": {
			Provider: "github",
			Repo:     "PapirusDevelopmentTeam/papirus-icon-theme",
			Ref:      "HEAD",
		},
	}

	got := BuildRenovateDependenciesFromLocks(sources, nil)
	if len(got) != 1 {
		t.Fatalf("expected 1 (lock state) dependency, got=%d (%#v)", len(got), got)
	}
	if got[0].Kind != "source" || got[0].DepName != "" {
		t.Fatalf("expected basic source lock state without renovate fields, got=%#v", got[0])
	}
}

func TestBuildRenovateDependenciesSkipsSHAOnlyWithoutBranch(t *testing.T) {
	t.Parallel()

	// Commit in URL but no named branch/tag => cannot satisfy git-refs.
	sources := map[string]LockedSource{
		"theme": {
			Provider: "github",
			Repo:     "catppuccin/gtk",
			URL:      "https://codeload.github.com/catppuccin/gtk/tar.gz/9aa0d1fabc1234",
		},
	}

	got := BuildRenovateDependenciesFromLocks(sources, nil)
	if len(got) != 1 {
		t.Fatalf("expected 1 dependency, got=%d", len(got))
	}
	if got[0].DepName != "" || got[0].Datasource != "" {
		t.Fatalf("expected no renovate fields without a named tracking ref, got=%#v", got[0])
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
			Kind:          "source",
			Ref:           "github:PapirusDevelopmentTeam/papirus-icon-theme",
			DepName:       "PapirusDevelopmentTeam/papirus-icon-theme",
			CurrentValue:  "master",
			CurrentDigest: "abc1234deadbeef",
			Datasource:    "git-refs",
			PackageName:   "https://github.com/PapirusDevelopmentTeam/papirus-icon-theme",
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
	src := byRef["github:PapirusDevelopmentTeam/papirus-icon-theme"]
	if src.CurrentValue != "master" || src.CurrentDigest != "abc1234deadbeef" {
		t.Fatalf("expected icons source to be preserved, got=%#v", src)
	}
}
