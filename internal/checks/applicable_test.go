package checks_test

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/lucasew/workspaced/internal/checks"
)

type stubCheck struct {
	name string
	err  error
}

func (s stubCheck) Name() string                         { return s.name }
func (s stubCheck) Detect(context.Context, string) error { return s.err }

func TestApplicable(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module x\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	detectErr := errors.New("boom")
	items := []checks.Check{
		stubCheck{name: "ok", err: nil},
		stubCheck{name: "skip", err: checks.ErrNotApplicable},
		stubCheck{name: "fail", err: detectErr},
	}

	var skips []string
	got := checks.Applicable(t.Context(), dir, items, func(name, reason string, err error) {
		skips = append(skips, name+":"+reason)
		if name == "fail" && !errors.Is(err, detectErr) {
			t.Errorf("fail skip err = %v, want %v", err, detectErr)
		}
	})

	if len(got) != 1 || got[0].Name() != "ok" {
		t.Fatalf("Applicable = %v, want [ok]", names(got))
	}
	if len(skips) != 2 || skips[0] != "skip:not applicable" || skips[1] != "fail:detect failed" {
		t.Fatalf("skips = %v", skips)
	}
}

func names(items []checks.Check) []string {
	out := make([]string, len(items))
	for i, item := range items {
		out[i] = item.Name()
	}
	return out
}
