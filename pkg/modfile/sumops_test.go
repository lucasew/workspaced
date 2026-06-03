package modfile

import "testing"

func TestSumFileEnsureToolIsIdempotent(t *testing.T) {
	t.Parallel()

	sum := &SumFile{
		Dependencies: []RenovateDependency{{
			Kind:         "tool",
			Name:         "gh",
			Ref:          "github:cli/cli",
			Version:      "2.89.0",
			Provider:     "github",
			DepName:      "cli/cli",
			CurrentValue: "2.89.0",
			Datasource:   "github-releases",
		}},
	}

	lock, ok := sum.Tool("gh")
	if !ok {
		t.Fatal("expected tool lock")
	}
	if lock.Version != "2.89.0" {
		t.Fatalf("unexpected version: %s", lock.Version)
	}

	if changed := sum.EnsureTool("gh", LockedTool{Ref: "github:cli/cli", Version: "2.89.0"}); changed {
		t.Fatal("expected EnsureTool to be idempotent when passing minimal (no renovate fields)")
	}
	if len(sum.Dependencies) != 1 {
		t.Fatalf("unexpected dependencies count: %d", len(sum.Dependencies))
	}
	d := sum.Dependencies[0]
	if d.Datasource != "github-releases" || d.DepName != "cli/cli" {
		t.Fatalf("renovate reference data should be preserved on the lock entry: %#v", d)
	}
}
