package checks

import (
	"os"
	"path/filepath"
	"testing"
)

func TestEvaluateDetectPath(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module x\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	res, err := EvaluateDetect(dir, map[string]DetectRule{
		"00-go": {Path: "go.mod", Enable: true},
	})
	if err != nil {
		t.Fatal(err)
	}
	if !res.Applicable || res.RuleKey != "00-go" {
		t.Fatalf("got %+v", res)
	}
}

func TestEvaluateDetectFirstMatchWins(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module x\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	res, err := EvaluateDetect(dir, map[string]DetectRule{
		"00-deny":  {Path: "go.mod", Enable: false},
		"01-allow": {Path: "go.mod", Enable: true},
	})
	if err != nil {
		t.Fatal(err)
	}
	if res.Applicable || res.RuleKey != "00-deny" {
		t.Fatalf("expected first deny, got %+v", res)
	}
}

func TestEvaluateDetectGlob(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "ok.sh"), []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(dir, "node_modules"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "node_modules", "x.sh"), []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	res, err := EvaluateDetect(dir, map[string]DetectRule{
		"00-sh": {Glob: "**/*.sh", Enable: true},
	})
	if err != nil {
		t.Fatal(err)
	}
	if !res.Applicable {
		t.Fatalf("expected applicable, got %+v", res)
	}
	files, err := CollectGlob(dir, "**/*.sh")
	if err != nil {
		t.Fatal(err)
	}
	if len(files) != 1 || files[0] != "ok.sh" {
		t.Fatalf("files=%v", files)
	}
}

func TestEvaluateDetectEmpty(t *testing.T) {
	t.Parallel()
	res, err := EvaluateDetect(t.TempDir(), nil)
	if err != nil {
		t.Fatal(err)
	}
	if res.Applicable {
		t.Fatal("empty detect should not apply")
	}
}
