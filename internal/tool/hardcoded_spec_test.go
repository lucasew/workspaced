package tool

import (
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"
)

// Product code must resolve tools through lazy_tools + the workspace lockfile.
// A literal spec@version (or implicit @latest via EnsureInstalled("github:…"))
// bypasses the lock. User-facing `tool with`/`which`/`install` take a spec
// from argv on purpose and are excluded.
func TestProductCodeDoesNotHardcodeToolSpecs(t *testing.T) {
	t.Parallel()

	root := moduleRoot(t)
	hardEnsure := regexp.MustCompile(`(?:EnsureAndRun|EnsureInstalled)\([^)]*"(?:github|registry|mise):[^"]+"`)
	goRunPin := regexp.MustCompile(`go",\s*"run",\s*"[^"]+@`)
	atLatest := regexp.MustCompile(`"(?:github|registry|mise):[^"]*@latest"`)

	var hits []string
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			name := d.Name()
			if name == ".git" || name == "dist" || name == "testdata" || name == "vendor" {
				return fs.SkipDir
			}
			return nil
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		if !hardcodedSpecScanFile(rel) {
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		for i, line := range strings.Split(string(data), "\n") {
			trim := strings.TrimSpace(line)
			if strings.HasPrefix(trim, "//") || strings.HasPrefix(trim, "#") {
				continue
			}
			if hardEnsure.MatchString(line) || goRunPin.MatchString(line) || atLatest.MatchString(line) {
				hits = append(hits, rel+":"+strconv.Itoa(i+1)+": "+trim)
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(hits) > 0 {
		t.Fatalf("hardcoded tool spec (use lazy_tools + lockfile):\n  %s", strings.Join(hits, "\n  "))
	}
}

func hardcodedSpecScanFile(rel string) bool {
	switch {
	case strings.HasSuffix(rel, "_test.go"):
		return false
	case strings.HasPrefix(rel, "cmd/workspaced/tool/"):
		return false
	case rel == "pkg/tool/tool.go":
		return false
	case rel == "internal/tool/resolver.go" || rel == "internal/tool/lazy.go" || rel == "internal/tool/lazy_exec.go":
		return false
	case strings.HasSuffix(rel, ".go") || strings.HasSuffix(rel, ".cue"):
		return strings.HasPrefix(rel, "cmd/") || strings.HasPrefix(rel, "internal/") || strings.HasPrefix(rel, "pkg/")
	default:
		return false
	}
}

func moduleRoot(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatal("go.mod not found")
		}
		dir = parent
	}
}
