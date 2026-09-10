package local

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/lucasew/workspaced/internal/module"
	"github.com/lucasew/workspaced/pkg/logging"
)

func TestResolvePresetBases(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	modulesDir := filepath.Join(root, "modules")
	modPath := filepath.Join(modulesDir, "demo")
	writeModuleTree(t, modPath, map[string]string{
		"home/.bashrc":         "home\n",
		"codebase/.gitignore":  "repo\n",
		"etc/nginx/nginx.conf": "nginx\n",
		"README.md":            "docs\n",
		"module.cue":           "package module\n\nmodule: { config: {} }\n",
	})

	home, err := os.UserHomeDir()
	if err != nil {
		t.Fatal(err)
	}

	got, err := (&Provider{}).Resolve(logging.NewWriterContext(t.Output()), module.ResolveRequest{
		Ref:            modPath,
		ModulesBaseDir: modulesDir,
	})
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}

	want := []module.ResolvedFile{
		{RelPath: ".gitignore", TargetBase: root},
		{RelPath: "nginx/nginx.conf", TargetBase: "/etc"},
		{RelPath: ".bashrc", TargetBase: home},
	}
	opts := []cmp.Option{
		cmpopts.IgnoreFields(module.ResolvedFile{}, "Mode", "Info", "AbsPath", "Symlink"),
		cmpopts.SortSlices(func(a, b module.ResolvedFile) bool {
			if a.TargetBase != b.TargetBase {
				return a.TargetBase < b.TargetBase
			}
			return a.RelPath < b.RelPath
		}),
	}
	if diff := cmp.Diff(want, got.Files, opts...); diff != "" {
		t.Fatalf("files mismatch (-want +got):\n%s", diff)
	}
}

func TestResolveUnknownPreset(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	modulesDir := filepath.Join(root, "modules")
	modPath := filepath.Join(modulesDir, "demo")
	writeModuleTree(t, modPath, map[string]string{
		"nope/file.txt": "x\n",
		"module.cue":    "package module\n\nmodule: { config: {} }\n",
	})

	_, err := (&Provider{}).Resolve(logging.NewWriterContext(t.Output()), module.ResolveRequest{
		Ref:            modPath,
		ModulesBaseDir: modulesDir,
	})
	if !errors.Is(err, ErrUnknownPreset) {
		t.Fatalf("err=%v want %v", err, ErrUnknownPreset)
	}
}

func TestResolvePresetBase(t *testing.T) {
	t.Parallel()

	home, err := os.UserHomeDir()
	if err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name           string
		preset         string
		modulesBaseDir string
		want           string
		wantErr        error
	}{
		{name: "home", preset: "home", modulesBaseDir: "/ws/modules", want: home},
		{name: "codebase", preset: "codebase", modulesBaseDir: "/ws/modules", want: "/ws"},
		{name: "etc", preset: "etc", modulesBaseDir: "/ws/modules", want: "/etc"},
		{name: "unknown", preset: "nope", modulesBaseDir: "/ws/modules", wantErr: ErrUnknownPreset},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, err := resolvePresetBase(tt.preset, tt.modulesBaseDir)
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("err=%v want %v", err, tt.wantErr)
			}
			if got != tt.want {
				t.Fatalf("base=%q want %q", got, tt.want)
			}
		})
	}
}

func writeModuleTree(t *testing.T, modPath string, files map[string]string) {
	t.Helper()
	for rel, content := range files {
		path := filepath.Join(modPath, rel)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
}
