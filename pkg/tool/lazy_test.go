package tool

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"testing"

	"workspaced/pkg/configcue"
	_ "workspaced/pkg/driver/env/native"
	"workspaced/pkg/logging"
	"workspaced/pkg/modfile"
	parsespec "workspaced/pkg/parse/spec"
	"workspaced/pkg/taskgroup"
)

func TestRefreshLazyToolLocksPreservesExistingLock(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	workspaceRoot := t.TempDir()
	writeTestFile(t, filepath.Join(workspaceRoot, "workspaced.cue"), `package workspaced

workspaced: {
	lazy_tools: {
		gh: {
			ref: "github:cli/cli"
			bins: ["gh"]
		}
	}
}
`)

	// Pin every lazy tool that the codebase prelude injects (plus gh from the
	// test cue) so refresh has nothing to resolve and must leave the lock untouched.
	writeTestFile(t, filepath.Join(workspaceRoot, "workspaced.lock.json"), `{
  "dependencies": [
    {"kind": "tool", "ref": "github:cli/cli", "currentValue": "0.1.0", "depName": "cli/cli", "datasource": "github-releases"},
    {"kind": "tool", "ref": "github:golangci/golangci-lint", "currentValue": "1.0.0", "depName": "golangci/golangci-lint", "datasource": "github-releases"},
    {"kind": "tool", "ref": "registry:shellcheck", "currentValue": "0.1.0", "depName": "shellcheck", "datasource": "github-releases"},
    {"kind": "tool", "ref": "github:astral-sh/ruff", "currentValue": "0.1.0", "depName": "astral-sh/ruff", "datasource": "github-releases"},
    {"kind": "tool", "ref": "github:rhysd/actionlint", "currentValue": "0.1.0", "depName": "rhysd/actionlint", "datasource": "github-releases"},
    {"kind": "tool", "ref": "registry:biome", "currentValue": "0.1.0", "depName": "biome", "datasource": "github-releases"},
    {"kind": "tool", "ref": "registry:nodejs", "currentValue": "0.1.0", "depName": "nodejs", "datasource": "github-releases"}
  ]
}
`)

	spec, err := parsespec.Parse("github:cli/cli")
	if err != nil {
		t.Fatalf("parse spec: %v", err)
	}
	binPath := filepath.Join(home, ".local", "share", "workspaced", "tools", spec.Dir(), "2.89.0", "bin", "gh")
	writeTestFile(t, binPath, "#!/bin/sh\nexit 0\n")
	if err := os.Chmod(binPath, 0o755); err != nil {
		t.Fatalf("chmod bin: %v", err)
	}

	g, ctx := taskgroup.New(logging.NewWriterContext(t.Output()), taskgroup.DefaultLimits())
	t.Cleanup(func() { _ = g.Wait() })
	cfg, err := configcue.LoadForWorkspace(ctx, workspaceRoot)
	if err != nil {
		t.Fatalf("load config: %v", err)
	}

	ws := modfile.NewWorkspace(workspaceRoot)
	before, err := os.ReadFile(ws.SumPath())
	if err != nil {
		t.Fatalf("read lock before: %v", err)
	}

	updated, err := RefreshLazyToolLocks(ctx, ws, cfg)
	if err != nil {
		t.Fatalf("refresh lazy tool locks: %v", err)
	}
	if updated != 0 {
		t.Fatalf("expected no tool locks updated, got %d", updated)
	}

	after, err := os.ReadFile(ws.SumPath())
	if err != nil {
		t.Fatalf("read lock after: %v", err)
	}
	if !bytes.Equal(before, after) {
		t.Fatalf("lockfile changed unexpectedly\nbefore:\n%s\nafter:\n%s", before, after)
	}
}

func TestApplyLiveToolEnrichmentIdempotent(t *testing.T) {
	t.Parallel()

	sum := &modfile.SumFile{
		Dependencies: []modfile.RenovateDependency{{
			Kind:         "tool",
			Ref:          "github:cli/cli",
			DepName:      "cli/cli",
			CurrentValue: "v2.95.0",
			Datasource:   "github-releases",
		}},
	}
	live := staticEnrichTool{
		depName:    "cli/cli",
		datasource: "github-releases",
	}
	if applyLiveToolEnrichment(sum, "github:cli/cli", "v2.95.0", live) {
		t.Fatal("expected no change when enrichment matches existing row")
	}
	if applyLiveToolEnrichment(sum, "github:cli/cli", "v2.95.0", staticEnrichTool{
		depName:     "cli/cli",
		datasource:  "github-releases",
		versioning:  "semver",
		extractVers: `^v(?<version>\d+)`,
	}) != true {
		t.Fatal("expected change when enrichment adds metadata")
	}
	if got := sum.Dependencies[0].Versioning; got != "semver" {
		t.Fatalf("Versioning = %q", got)
	}
	if applyLiveToolEnrichment(sum, "github:other/other", "1.0.0", nil) != true {
		t.Fatal("expected change when creating missing row")
	}
	if applyLiveToolEnrichment(sum, "github:other/other", "1.0.0", nil) {
		t.Fatal("expected create to be idempotent on second call")
	}
}

type staticEnrichTool struct {
	depName     string
	datasource  string
	versioning  string
	extractVers string
}

func (t staticEnrichTool) ListVersions(context.Context) ([]string, error) { return nil, nil }
func (t staticEnrichTool) Install(context.Context, string, string) error  { return nil }
func (t staticEnrichTool) EnrichLockfile(entry *modfile.RenovateDependency) {
	if t.depName != "" {
		entry.DepName = t.depName
	}
	if t.datasource != "" {
		entry.Datasource = t.datasource
	}
	if t.versioning != "" {
		entry.Versioning = t.versioning
	}
	if t.extractVers != "" {
		entry.ExtractVersion = t.extractVers
	}
}

func writeTestFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", path, err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}
