package modfile

import "testing"

func TestSumFileEnsureToolIsIdempotent(t *testing.T) {
	t.Parallel()

	sum := &SumFile{
		Dependencies: []RenovateDependency{{
			Kind:    "tool",
			Name:    "gh",
			Ref:     "github:cli/cli",
			Version: "2.89.0",
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
		t.Fatal("expected EnsureTool to be idempotent")
	}
	if len(sum.Dependencies) != 1 {
		t.Fatalf("unexpected dependencies count: %d", len(sum.Dependencies))
	}
}
