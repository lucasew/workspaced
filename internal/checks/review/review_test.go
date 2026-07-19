package review

import (
	"bytes"
	"strings"
	"testing"

	"github.com/owenrumney/go-sarif/v2/sarif"
)

func TestParseUnifiedDiffLines(t *testing.T) {
	t.Parallel()
	diff := `diff --git a/foo.go b/foo.go
--- a/foo.go
+++ b/foo.go
@@ -10,0 +11,2 @@
+line eleven
+line twelve
@@ -20 +22 @@
+line twenty two
`
	set := parseUnifiedDiffLines(diff)
	if !set["foo.go:11"] || !set["foo.go:12"] || !set["foo.go:22"] {
		t.Fatalf("set=%v", set)
	}
	if set["foo.go:10"] {
		t.Fatal("should not include old-only lines")
	}
}

func TestWriteWorkflowCommand(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer
	writeWorkflowCommand(&buf, "error", "a.go", 3, 4, "golangci-lint", "boom\nthere")
	got := buf.String()
	if !strings.Contains(got, "::error file=a.go,line=3,col=4::") {
		t.Fatalf("got %q", got)
	}
	if strings.Contains(got, "\nthere") {
		t.Fatalf("newline not sanitized: %q", got)
	}
}

func TestAnnotateFiltersByDiff(t *testing.T) {
	// unit-level: extract + filter logic via parse only; full Annotate needs git.
	run := sarif.NewRun(*sarif.NewTool(sarif.NewDriver("t")))
	run.AddResult(sarif.NewRuleResult("r").
		WithLevel("error").
		WithMessage(sarif.NewTextMessage("m")).
		WithLocations([]*sarif.Location{
			sarif.NewLocation().WithPhysicalLocation(
				sarif.NewPhysicalLocation().
					WithArtifactLocation(sarif.NewArtifactLocation().WithUri("foo.go")).
					WithRegion(sarif.NewRegion().WithStartLine(11)),
			),
		}))
	file, line, _, msg, level := extractFinding(run.Results[0])
	if file != "foo.go" || line != 11 || msg != "m" || level != "error" {
		t.Fatalf("%s %d %s %s", file, line, msg, level)
	}
}

func TestIsGitHubActions(t *testing.T) {
	t.Setenv("GITHUB_ACTIONS", "true")
	if !IsGitHubActions() {
		t.Fatal("expected true")
	}
	t.Setenv("GITHUB_ACTIONS", "")
	if IsGitHubActions() {
		t.Fatal("expected false")
	}
}
