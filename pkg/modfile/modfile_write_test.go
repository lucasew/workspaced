package modfile

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestWriteModFile(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "workspaced.mod.toml")
	err := WriteModFile(path, &ModFile{
		Sources: map[string]SourceConfig{
			"remote": {Provider: "github", Repo: "lucasew/workspaced-modules"},
		},
	})
	if err != nil {
		t.Fatalf("write mod file: %v", err)
	}

	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read mod file: %v", err)
	}
	s := string(b)
	if !strings.Contains(s, "[sources]") {
		t.Fatalf("missing sources section: %s", s)
	}
	if !strings.Contains(s, `remote = "github:lucasew/workspaced-modules"`) {
		t.Fatalf("missing source spec entry: %s", s)
	}
	if strings.Contains(s, "[modules]") {
		t.Fatalf("unexpected modules section: %s", s)
	}
}
