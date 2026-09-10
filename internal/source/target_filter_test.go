package source

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/lucasew/workspaced/internal/configcue"
	_ "github.com/lucasew/workspaced/internal/module/prelude"
	_ "github.com/lucasew/workspaced/pkg/driver/env/native"
	"github.com/lucasew/workspaced/pkg/logging"
	"github.com/lucasew/workspaced/pkg/taskgroup"
)

func TestKeepTargetPlugin(t *testing.T) {
	t.Parallel()
	repo := t.TempDir()
	home := filepath.Join(t.TempDir(), "home")
	files := []File{
		&BufferFile{BasicFile: BasicFile{RelPathStr: ".gitignore", TargetBaseDir: repo}},
		&BufferFile{BasicFile: BasicFile{RelPathStr: ".bashrc", TargetBaseDir: home}},
		&BufferFile{BasicFile: BasicFile{RelPathStr: "nginx.conf", TargetBaseDir: "/etc"}},
	}

	got, err := NewKeepTargetPlugin(repo).Process(logging.NewWriterContext(t.Output()), files)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{filepath.Join(repo, ".gitignore")}
	if diff := cmp.Diff(want, fileTargets(got)); diff != "" {
		t.Fatalf("keep mismatch (-want +got):\n%s", diff)
	}
}

func TestDropTargetPlugin(t *testing.T) {
	t.Parallel()
	repo := t.TempDir()
	home := filepath.Join(t.TempDir(), "home")
	files := []File{
		&BufferFile{BasicFile: BasicFile{RelPathStr: ".gitignore", TargetBaseDir: repo}},
		&BufferFile{BasicFile: BasicFile{RelPathStr: ".bashrc", TargetBaseDir: home}},
	}

	got, err := NewDropTargetPlugin(repo).Process(logging.NewWriterContext(t.Output()), files)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{filepath.Join(home, ".bashrc")}
	if diff := cmp.Diff(want, fileTargets(got)); diff != "" {
		t.Fatalf("drop mismatch (-want +got):\n%s", diff)
	}
}

func TestStandardProvidersTargetFilters(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		opts StandardDotfilesOptions
		want []string
	}{
		{
			name: "keep",
			opts: StandardDotfilesOptions{KeepTarget: "/repo"},
			want: []string{"keep-target"},
		},
		{
			name: "drop",
			opts: StandardDotfilesOptions{DropTarget: "/repo"},
			want: []string{"drop-target"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			providers, err := standardProviders(tt.opts)
			if err != nil {
				t.Fatal(err)
			}
			got := make([]string, len(providers))
			for i, p := range providers {
				got[i] = p.Name()
			}
			if diff := cmp.Diff(tt.want, got); diff != "" {
				t.Fatalf("providers mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestStandardDotfilesKeepsCodebasePresetOnly(t *testing.T) {
	root := t.TempDir()
	modDir := filepath.Join(root, "modules", "demo")
	writeFile(t, filepath.Join(modDir, "module.cue"), "package module\n\nmodule: { config: {} }\n")
	writeFile(t, filepath.Join(modDir, "home", ".bashrc"), "from-home\n")
	writeFile(t, filepath.Join(modDir, "codebase", ".gitignore"), "from-codebase\n")
	writeFile(t, filepath.Join(root, "workspaced.cue"), `package workspaced

workspaced: {
	modules: {
		demo: {
			enable: true
			input: "self"
			path: "modules/demo"
		}
	}
}
`)
	writeFile(t, filepath.Join(root, "workspaced.lock.json"), `{"dependencies":[]}`)

	g, ctx := taskgroup.New(logging.NewWriterContext(t.Output()), taskgroup.DefaultLimits())
	t.Cleanup(func() {
		if err := g.Wait(); err != nil && !t.Failed() {
			t.Errorf("group wait: %v", err)
		}
	})
	cfg, err := configcue.LoadForWorkspace(ctx, root)
	if err != nil {
		t.Fatalf("load config: %v", err)
	}

	t.Run("keep codebase", func(t *testing.T) {
		b, err := StandardDotfilesOptions{
			ConfigTreeTarget: root,
			KeepTarget:       root,
			ModulesDir:       filepath.Join(root, "modules"),
			ModulesCfg:       cfg,
		}.Builder(cfg)
		if err != nil {
			t.Fatal(err)
		}
		tree, err := b.Tree(ctx)
		if err != nil {
			t.Fatal(err)
		}
		got := fileTargets(tree.Files())
		want := []string{filepath.Join(root, ".gitignore")}
		if diff := cmp.Diff(want, got); diff != "" {
			t.Fatalf("codebase files mismatch (-want +got):\n%s", diff)
		}
	})

	t.Run("drop codebase", func(t *testing.T) {
		home, err := os.UserHomeDir()
		if err != nil {
			t.Fatal(err)
		}
		b, err := StandardDotfilesOptions{
			ConfigTreeTarget: home,
			DropTarget:       root,
			ModulesDir:       filepath.Join(root, "modules"),
			ModulesCfg:       cfg,
		}.Builder(cfg)
		if err != nil {
			t.Fatal(err)
		}
		tree, err := b.Tree(ctx)
		if err != nil {
			t.Fatal(err)
		}
		got := fileTargets(tree.Files())
		want := []string{filepath.Join(home, ".bashrc")}
		if diff := cmp.Diff(want, got); diff != "" {
			t.Fatalf("home files mismatch (-want +got):\n%s", diff)
		}
	})
}

func fileTargets(files []File) []string {
	out := make([]string, len(files))
	for i, f := range files {
		out[i] = filepath.Join(f.TargetBase(), f.RelPath())
	}
	return out
}
