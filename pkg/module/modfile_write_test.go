package module

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
		Modules: map[string]string{
			"zeta": "remote:path/zeta",
			"alfa": "core:base16-icons-linux",
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
	if !strings.Contains(s, "[sources.remote]") {
		t.Fatalf("missing source section: %s", s)
	}
	if !strings.Contains(s, "[modules]") {
		t.Fatalf("missing modules section: %s", s)
	}
	if strings.Index(s, `alfa = "core:base16-icons-linux"`) > strings.Index(s, `zeta = "remote:path/zeta"`) {
		t.Fatalf("modules are not sorted: %s", s)
	}
}
