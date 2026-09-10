package filespine

import (
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
)

func TestParseRootModeFilters(t *testing.T) {
	t.Parallel()
	fileVal := compileFileValue(t, `
file: {
	".codex/config.toml": {type: "toml", values: {model: "x"}}
	home: {
		".bashrc": {type: "text", values: {content: "umask 022"}}
	}
	codebase: {
		".gitignore": {type: "text", values: {content: "bin/"}}
	}
	etc: {
		"hosts": {type: "text", values: {content: "127.0.0.1 localhost"}}
	}
}
`)

	tests := []struct {
		mode string
		want []string
	}{
		{mode: ModeHome, want: []string{".bashrc", ".codex/config.toml", "hosts"}},
		{mode: ModeCodebase, want: []string{".gitignore"}},
		{mode: ModeSystem, want: nil},
	}
	for _, tt := range tests {
		t.Run(tt.mode, func(t *testing.T) {
			t.Parallel()
			got, err := ParseRoot(fileVal, ParseRootOptions{Mode: tt.mode})
			if err != nil {
				t.Fatal(err)
			}
			keys := make([]string, 0, len(got))
			for k := range got {
				keys = append(keys, k)
			}
			if diff := cmp.Diff(tt.want, keys, cmpopts.SortSlices(func(a, b string) bool { return a < b }), cmpopts.EquateEmpty()); diff != "" {
				t.Fatalf("keys mismatch (-want +got):\n%s", diff)
			}
			if tt.mode == ModeHome {
				if got["hosts"].TargetBase != "/etc" {
					t.Fatalf("etc TargetBase=%q", got["hosts"].TargetBase)
				}
				if got[".bashrc"].TargetBase != "" {
					t.Fatalf("home TargetBase=%q want empty", got[".bashrc"].TargetBase)
				}
			}
		})
	}
}
