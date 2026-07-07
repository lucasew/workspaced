package ruff

import (
	"os"
	"path/filepath"
	"testing"

	_ "workspaced/internal/tool/backend/catalog"
	_ "workspaced/internal/tool/backend/catalog/applications"
	_ "workspaced/internal/tool/backend/github"
	_ "workspaced/pkg/driver/httpclient/native"
	"workspaced/pkg/logging"
)

func TestRun(t *testing.T) {
	if os.Getenv("CI") != "" {
		t.Skip("Skipping integration test in CI")
	}

	dir := t.TempDir()

	if err := os.WriteFile(filepath.Join(dir, "uv.lock"), []byte(""), 0644); err != nil {
		t.Fatal(err)
	}

	pyFile := filepath.Join(dir, "main.py")
	if err := os.WriteFile(pyFile, []byte("import os\n"), 0644); err != nil {
		t.Fatal(err)
	}

	p := New()
	ctx := logging.NewWriterContext(t.Output())

	err := p.Detect(ctx, dir)
	if err != nil {
		t.Fatalf("Detect failed: %v", err)
	}

	run, err := p.Run(ctx, dir)
	if err != nil {
		t.Skipf("ruff unavailable in this environment: %v", err)
	}

	if run == nil {
		t.Fatal("Run returned nil report")
	}

	// We expect at least one result (unused import 'os')
	if len(run.Results) == 0 {
		t.Fatal("Expected lint results, got none")
	}

	found := false
	for _, res := range run.Results {
		if res.RuleID != nil && *res.RuleID == "F401" {
			found = true
			break
		}
	}

	if !found {
		t.Log("Results found:")
		for _, res := range run.Results {
			if res.RuleID != nil {
				t.Logf("- %s", *res.RuleID)
			}
		}
		t.Error("Did not find F401 (unused import) in results")
	}
}
