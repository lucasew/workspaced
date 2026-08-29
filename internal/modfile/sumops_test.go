package modfile

import "testing"

func TestSumFileEnsureToolIsIdempotent(t *testing.T) {
	t.Parallel()

	sum := &SumFile{}
	sum.EnsureTool("gh", LockedTool{
		Ref:        "github:cli/cli",
		Version:    "2.89.0",
		DepName:    "cli/cli",
		Datasource: "github-releases",
	})

	lock, ok := sum.Tool("github:cli/cli")
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

func TestUpsertToolRefreshesStaleCurrentValue(t *testing.T) {
	sum := &SumFile{}
	sum.EnsureTool("tirith", LockedTool{
		Ref:        "registry:tirith",
		Version:    "v0.3.1",
		DepName:    "sheeki03/tirith",
		Datasource: "github-releases",
	})

	changed := sum.EnsureTool("tirith", LockedTool{
		Ref:        "registry:tirith",
		Version:    "v0.3.1",
		DepName:    "sheeki03/tirith",
		Datasource: "github-releases",
		Versioning: "semver",
	})
	if !changed {
		t.Fatal("expected stale currentValue to be refreshed")
	}

	dep := sum.Dependencies[0]
	if dep.CurrentValue != "v0.3.1" {
		t.Fatalf("CurrentValue = %q, want %q", dep.CurrentValue, "v0.3.1")
	}
	if dep.Versioning != "semver" {
		t.Fatalf("Versioning = %q, want semver", dep.Versioning)
	}
}
