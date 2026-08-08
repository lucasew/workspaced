package checks

import (
	"testing"

	"github.com/owenrumney/go-sarif/v2/sarif"
)

func TestBundleRunsSkipsNilAndPreservesOrder(t *testing.T) {
	r1 := sarif.NewRun(*sarif.NewTool(sarif.NewDriver("a")))
	r2 := sarif.NewRun(*sarif.NewTool(sarif.NewDriver("b")))
	report, err := BundleRuns(r1, nil, r2)
	if err != nil {
		t.Fatal(err)
	}
	if len(report.Runs) != 2 {
		t.Fatalf("runs=%d want 2", len(report.Runs))
	}
	if report.Runs[0].Tool.Driver.Name != "a" || report.Runs[1].Tool.Driver.Name != "b" {
		t.Fatalf("order/names: %#v %#v", report.Runs[0].Tool.Driver.Name, report.Runs[1].Tool.Driver.Name)
	}
}

func TestBundleRunsEmpty(t *testing.T) {
	report, err := BundleRuns()
	if err != nil {
		t.Fatal(err)
	}
	if report == nil || len(report.Runs) != 0 {
		t.Fatalf("expected empty report, got %#v", report)
	}
}
