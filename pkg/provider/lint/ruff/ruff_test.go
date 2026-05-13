package ruff

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	_ "workspaced/pkg/driver/httpclient/native"
	_ "workspaced/pkg/tool/provider/github"
)

func TestRun(t *testing.T) {
	if os.Getenv("CI") != "" {
		t.Skip("Skipping integration test in CI")
	}

	// Create a temporary directory for the test project
	dir := t.TempDir()

	// Create uv.lock to trigger detection
	if err := os.WriteFile(filepath.Join(dir, "uv.lock"), []byte(""), 0644); err != nil {
		t.Fatal(err)
	}

	// Create a python file with lint errors (unused import)
	pyFile := filepath.Join(dir, "main.py")
	if err := os.WriteFile(pyFile, []byte("import os\n"), 0644); err != nil {
		t.Fatal(err)
	}

	p := New()
	ctx := context.Background()

	// Verify detection
	err := p.Detect(ctx, dir)
	if err != nil {
		t.Fatalf("Detect failed: %v", err)
	}

	// Run lint
	run, err := p.Run(ctx, dir)
	if err != nil {
		t.Fatalf("Run failed: %v", err)
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
