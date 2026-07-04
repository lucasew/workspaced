package checks_test

import (
	"testing"

	"workspaced/pkg/checks"
)

func TestFirstSARIFRun(t *testing.T) {
	t.Parallel()

	t.Run("empty runs", func(t *testing.T) {
		t.Parallel()
		run, err := checks.FirstSARIFRun([]byte(`{"version":"2.1.0","runs":[]}`))
		if err != nil {
			t.Fatal(err)
		}
		if run != nil {
			t.Fatalf("got run %v, want nil", run)
		}
	})

	t.Run("first run", func(t *testing.T) {
		t.Parallel()
		run, err := checks.FirstSARIFRun([]byte(`{
			"version":"2.1.0",
			"runs":[{"tool":{"driver":{"name":"demo"}},"results":[]}]
		}`))
		if err != nil {
			t.Fatal(err)
		}
		if run == nil || run.Tool.Driver.Name != "demo" {
			t.Fatalf("unexpected run: %#v", run)
		}
	})

	t.Run("invalid", func(t *testing.T) {
		t.Parallel()
		_, err := checks.FirstSARIFRun([]byte(`not-json`))
		if err == nil {
			t.Fatal("expected parse error")
		}
	})
}

func TestStringPtr(t *testing.T) {
	t.Parallel()
	p := checks.StringPtr("uri")
	if p == nil || *p != "uri" {
		t.Fatalf("got %#v", p)
	}
}
