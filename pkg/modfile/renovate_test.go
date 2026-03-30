package modfile

import "testing"

func TestBuildRenovateDependenciesFromTools(t *testing.T) {
	t.Parallel()

	sum := &SumFile{
		Sources: map[string]LockedSource{
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
		},
		Tools: map[string]LockedTool{
			"fd":   {Ref: "github:sharkdp/fd", Version: "v10.3.0"},
			"fzf":  {Ref: "mise:fzf", Version: "0.50.0"},
			"bad1": {Ref: "", Version: "1.0.0"},
			"bad2": {Ref: "github:foo/bar", Version: ""},
		},
	}

	got := BuildRenovateDependencies(sum)
	if len(got) != 3 {
		t.Fatalf("expected 3 dependencies, got=%d", len(got))
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

	sum := &SumFile{
		Sources: map[string]LockedSource{
			"icons": {
				Provider: "github",
				Repo:     "PapirusDevelopmentTeam/papirus-icon-theme",
				Ref:      "HEAD",
			},
		},
	}

	got := BuildRenovateDependencies(sum)
	if len(got) != 0 {
		t.Fatalf("expected 0 dependencies, got=%d (%#v)", len(got), got)
	}
}

func TestMergeRenovateDependenciesPreservesUntouchedEntries(t *testing.T) {
	t.Parallel()

	existing := []RenovateDependency{
		{
			Kind:         "tool",
			Name:         "fd",
			Provider:     "github",
			Ref:          "github:sharkdp/fd",
			Version:      "v10.2.0",
			DepName:      "sharkdp/fd",
			CurrentValue: "v10.2.0",
			Datasource:   "github-releases",
		},
		{
			Kind:         "source",
			Name:         "icons",
			Provider:     "github",
			Repo:         "PapirusDevelopmentTeam/papirus-icon-theme",
			Ref:          "v2026.02.01",
			DepName:      "PapirusDevelopmentTeam/papirus-icon-theme",
			CurrentValue: "v2026.02.01",
			Datasource:   "github-tags",
		},
	}

	generated := []RenovateDependency{
		{
			Kind:         "tool",
			Name:         "fd",
			Provider:     "github",
			Ref:          "github:sharkdp/fd",
			Version:      "v10.3.0",
			DepName:      "sharkdp/fd",
			CurrentValue: "v10.3.0",
			Datasource:   "github-releases",
		},
	}

	got := MergeRenovateDependencies(existing, generated)
	if len(got) != 2 {
		t.Fatalf("expected 2 dependencies after merge, got=%d", len(got))
	}

	byName := map[string]RenovateDependency{}
	for _, dep := range got {
		byName[dep.Name] = dep
	}
	if byName["fd"].CurrentValue != "v10.3.0" {
		t.Fatalf("expected fd to be updated, got=%q", byName["fd"].CurrentValue)
	}
	if byName["icons"].CurrentValue != "v2026.02.01" {
		t.Fatalf("expected icons to be preserved, got=%q", byName["icons"].CurrentValue)
	}
}
