package shellcheck

import (
	"os"
	"path/filepath"
	"testing"

	_ "workspaced/pkg/driver/httpclient/native"
	"workspaced/pkg/logging"
	_ "workspaced/pkg/tool/backend/catalog"
	_ "workspaced/pkg/tool/backend/catalog/applications"
	_ "workspaced/pkg/tool/backend/github"
)

func TestDetectSkipsNonShellDirs(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "readme.md"), []byte("hi"), 0o644); err != nil {
		t.Fatal(err)
	}

	p := New()
	if err := p.Detect(t.Context(), dir); err == nil {
		t.Fatal("expected ErrNotApplicable without .sh files")
	}
}

func TestDetectFindsShFiles(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "setup.sh"), []byte("#!/bin/sh\necho ok\n"), 0o755); err != nil {
		t.Fatal(err)
	}

	p := New()
	if err := p.Detect(t.Context(), dir); err != nil {
		t.Fatalf("Detect: %v", err)
	}
}

func TestRunReportsIssues(t *testing.T) {
	if os.Getenv("CI") != "" {
		t.Skip("Skipping integration test in CI")
	}

	dir := t.TempDir()
	script := filepath.Join(dir, "bad.sh")
	// Unquoted variable: SC2086 / SC2154 depending on version.
	if err := os.WriteFile(script, []byte("#!/bin/bash\necho $unquoted\n"), 0o755); err != nil {
		t.Fatal(err)
	}

	p := New()
	ctx := logging.NewWriterContext(t.Output())

	if err := p.Detect(ctx, dir); err != nil {
		t.Fatalf("Detect: %v", err)
	}

	run, err := p.Run(ctx, dir)
	if err != nil {
		t.Skipf("shellcheck unavailable in this environment: %v", err)
	}
	if run == nil || len(run.Results) == 0 {
		t.Fatal("expected lint results, got none")
	}

	found := false
	for _, res := range run.Results {
		if res.RuleID != nil && (*res.RuleID == "SC2086" || *res.RuleID == "SC2154") {
			found = true
			break
		}
	}
	if !found {
		for _, res := range run.Results {
			if res.RuleID != nil {
				t.Logf("- %s", *res.RuleID)
			}
		}
		t.Error("expected SC2086 or SC2154 in results")
	}
}

func TestConvertToSarifMapsLevels(t *testing.T) {
	t.Parallel()

	run := convertToSarif([]Issue{
		{File: "a.sh", Line: 1, Column: 1, EndLine: 1, EndColumn: 2, Level: "error", Code: 1001, Message: "err"},
		{File: "a.sh", Line: 2, Column: 1, EndLine: 2, EndColumn: 2, Level: "style", Code: 1002, Message: "style"},
		{File: "a.sh", Line: 3, Column: 1, EndLine: 3, EndColumn: 2, Level: "warning", Code: 1003, Message: "warn"},
	})
	if run == nil || len(run.Results) != 3 {
		t.Fatalf("expected 3 results, got %v", run)
	}
	levels := []string{}
	for _, r := range run.Results {
		if r.Level != nil {
			levels = append(levels, *r.Level)
		}
	}
	if levels[0] != "error" || levels[1] != "note" || levels[2] != "warning" {
		t.Fatalf("unexpected levels: %v", levels)
	}
}

func TestCollectShellFilesSkipsVendorDirs(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "node_modules", "pkg"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "node_modules", "pkg", "x.sh"), []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "ok.sh"), []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}

	files, err := collectShellFiles(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(files) != 1 || files[0] != "ok.sh" {
		t.Fatalf("collectShellFiles() = %v, want [ok.sh]", files)
	}
}
