package codec

import (
	"encoding/json"
	"testing"
)

func TestParseAndConvert(t *testing.T) {
	input := []eslintResult{
		{
			FilePath: "/path/to/file.js",
			Messages: []eslintMessage{
				{RuleID: "no-unused-vars", Severity: 2, Message: "Unused variable", Line: 1, Column: 5, EndLine: 1, EndColumn: 10},
				{RuleID: "no-console", Severity: 1, Message: "Unexpected console statement", Line: 10, Column: 1, EndLine: 10, EndColumn: 10},
			},
		},
	}
	jsonBytes, err := json.Marshal(input)
	if err != nil {
		t.Fatal(err)
	}
	run, err := parseAndConvertESLint(jsonBytes)
	if err != nil {
		t.Fatal(err)
	}
	if len(run.Results) != 2 {
		t.Fatalf("expected 2 results, got %d", len(run.Results))
	}
	if *run.Results[0].RuleID != "no-unused-vars" || *run.Results[0].Level != "error" {
		t.Fatalf("res0: %+v", run.Results[0])
	}
	if *run.Results[1].RuleID != "no-console" || *run.Results[1].Level != "warning" {
		t.Fatalf("res1: %+v", run.Results[1])
	}
}

func TestParseAndConvert_WithRawNewlineInString(t *testing.T) {
	raw := []byte(`[{"filePath":"/tmp/a.js","messages":[{"ruleId":"x","severity":2,"message":"line1
line2","line":1,"column":1,"endLine":1,"endColumn":2}]}]`)
	run, err := parseAndConvertESLint(raw)
	if err != nil {
		t.Fatal(err)
	}
	if len(run.Results) != 1 {
		t.Fatalf("got %d", len(run.Results))
	}
	if run.Results[0].Message.Text == nil || *run.Results[0].Message.Text != "line1\nline2" {
		t.Fatalf("msg=%v", run.Results[0].Message.Text)
	}
}

func TestParseAndConvert_WithNonJSONPrefix(t *testing.T) {
	raw := []byte(`note: running eslint
[{"filePath":"/tmp/b.js","messages":[{"ruleId":"y","severity":1,"message":"ok","line":2,"column":3,"endLine":2,"endColumn":4}]}]
done`)
	run, err := parseAndConvertESLint(raw)
	if err != nil {
		t.Fatal(err)
	}
	if len(run.Results) != 1 || run.Results[0].RuleID == nil || *run.Results[0].RuleID != "y" {
		t.Fatalf("got %+v", run)
	}
}

func TestParseAndConvert_WithTruncatedJSONTail(t *testing.T) {
	raw := []byte(`[{"filePath":"/tmp/ok.js","messages":[{"ruleId":"ok","severity":1,"message":"first","line":1,"column":1,"endLine":1,"endColumn":2}]},{"filePath":"/tmp/broken.js","messages":[`)
	run, err := parseAndConvertESLint(raw)
	if err != nil {
		t.Fatal(err)
	}
	if len(run.Results) != 1 || *run.Results[0].RuleID != "ok" {
		t.Fatalf("got %+v", run)
	}
}
