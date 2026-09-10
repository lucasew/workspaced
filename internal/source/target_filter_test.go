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
	cfgCode, err := configcue.LoadForWorkspace(ctx, root)
	if err != nil {
		t.Fatalf("load codebase config: %v", err)
	}
	cfgHome, err := configcue.LoadFiles(ctx, []string{filepath.Join(root, "workspaced.cue")})
	if err != nil {
		t.Fatalf("load home config: %v", err)
	}

	t.Run("codebase mode", func(t *testing.T) {
		b, err := StandardDotfilesOptions{
			ConfigTreeTarget: root,
			ModulesDir:       filepath.Join(root, "modules"),
			ModulesCfg:       cfgCode,
		}.Builder(cfgCode)
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

	t.Run("home mode", func(t *testing.T) {
		home, err := os.UserHomeDir()
		if err != nil {
			t.Fatal(err)
		}
		b, err := StandardDotfilesOptions{
			ConfigTreeTarget: home,
			ModulesDir:       filepath.Join(root, "modules"),
			ModulesCfg:       cfgHome,
		}.Builder(cfgHome)
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
