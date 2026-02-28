package eslint

import (
	"encoding/json"
	"testing"
)

func TestParseAndConvert(t *testing.T) {
	input := []Result{
		{
			FilePath: "/path/to/file.js",
			Messages: []Message{
				{
					RuleID:    "no-unused-vars",
					Severity:  2,
					Message:   "Unused variable",
					Line:      1,
					Column:    5,
					EndLine:   1,
					EndColumn: 10,
				},
				{
					RuleID:    "no-console",
					Severity:  1,
					Message:   "Unexpected console statement",
					Line:      10,
					Column:    1,
					EndLine:   10,
					EndColumn: 10,
				},
			},
		},
	}

	jsonBytes, err := json.Marshal(input)
	if err != nil {
		t.Fatalf("failed to marshal input: %v", err)
	}

	run, err := parseAndConvert(jsonBytes)
	if err != nil {
		t.Fatalf("parseAndConvert failed: %v", err)
	}

	if len(run.Results) != 2 {
		t.Errorf("expected 2 results, got %d", len(run.Results))
	}

	res1 := run.Results[0]
	if *res1.RuleID != "no-unused-vars" {
		t.Errorf("expected ruleId no-unused-vars, got %s", *res1.RuleID)
	}
	if *res1.Level != "error" {
		t.Errorf("expected level error, got %s", *res1.Level)
	}

	// Check location
	loc := res1.Locations[0]
	if *loc.PhysicalLocation.ArtifactLocation.URI != "/path/to/file.js" {
		t.Errorf("expected uri /path/to/file.js, got %s", *loc.PhysicalLocation.ArtifactLocation.URI)
	}
	if *loc.PhysicalLocation.Region.StartLine != 1 {
		t.Errorf("expected start line 1, got %d", *loc.PhysicalLocation.Region.StartLine)
	}

	res2 := run.Results[1]
	if *res2.RuleID != "no-console" {
		t.Errorf("expected ruleId no-console, got %s", *res2.RuleID)
	}
	if *res2.Level != "warning" {
		t.Errorf("expected level warning, got %s", *res2.Level)
	}
}

func TestParseAndConvert_WithRawNewlineInString(t *testing.T) {
	// Simulates broken JSON payload where a message contains a raw newline.
	raw := []byte(`[{"filePath":"/tmp/a.js","messages":[{"ruleId":"x","severity":2,"message":"line1
line2","line":1,"column":1,"endLine":1,"endColumn":2}]}]`)

	run, err := parseAndConvert(raw)
	if err != nil {
		t.Fatalf("parseAndConvert failed: %v", err)
	}

	if len(run.Results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(run.Results))
	}

	if run.Results[0].Message.Text == nil || *run.Results[0].Message.Text != "line1\nline2" {
		t.Fatalf("unexpected message: %+v", run.Results[0].Message.Text)
	}
}

func TestParseAndConvert_WithNonJSONPrefix(t *testing.T) {
	raw := []byte(`note: running eslint
[{"filePath":"/tmp/b.js","messages":[{"ruleId":"y","severity":1,"message":"ok","line":2,"column":3,"endLine":2,"endColumn":4}]}]
done`)

	run, err := parseAndConvert(raw)
	if err != nil {
		t.Fatalf("parseAndConvert failed: %v", err)
	}

	if len(run.Results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(run.Results))
	}

	if run.Results[0].RuleID == nil || *run.Results[0].RuleID != "y" {
		t.Fatalf("unexpected rule id: %+v", run.Results[0].RuleID)
	}
}

func TestParseAndConvert_WithTruncatedJSONTail(t *testing.T) {
	raw := []byte(`[{"filePath":"/tmp/ok.js","messages":[{"ruleId":"ok","severity":1,"message":"first","line":1,"column":1,"endLine":1,"endColumn":2}]},{"filePath":"/tmp/broken.js","messages":[`)

	run, err := parseAndConvert(raw)
	if err != nil {
		t.Fatalf("parseAndConvert failed: %v", err)
	}

	if len(run.Results) != 1 {
		t.Fatalf("expected 1 recovered result, got %d", len(run.Results))
	}

	if run.Results[0].RuleID == nil || *run.Results[0].RuleID != "ok" {
		t.Fatalf("unexpected rule id: %+v", run.Results[0].RuleID)
	}
}
