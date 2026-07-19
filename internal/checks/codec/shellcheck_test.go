package codec

import (
	"testing"
)

func TestDecodeShellcheckLevels(t *testing.T) {
	t.Parallel()
	raw := []byte(`[
		{"file":"a.sh","line":1,"column":1,"endLine":1,"endColumn":2,"level":"error","code":1001,"message":"err"},
		{"file":"a.sh","line":2,"column":1,"endLine":2,"endColumn":2,"level":"style","code":1002,"message":"style"},
		{"file":"a.sh","line":3,"column":1,"endLine":3,"endColumn":2,"level":"warning","code":1003,"message":"warn"}
	]`)
	run, err := decodeShellcheck("shellcheck", raw)
	if err != nil {
		t.Fatal(err)
	}
	if run == nil || len(run.Results) != 3 {
		t.Fatalf("got %v", run)
	}
	levels := []string{}
	for _, r := range run.Results {
		if r.Level != nil {
			levels = append(levels, *r.Level)
		}
	}
	if levels[0] != "error" || levels[1] != "note" || levels[2] != "warning" {
		t.Fatalf("levels=%v", levels)
	}
}
