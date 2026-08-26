package dotfiles

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/lucasew/workspaced/internal/deployer"
	"github.com/lucasew/workspaced/internal/source"
	"github.com/lucasew/workspaced/pkg/logging"
	"github.com/lucasew/workspaced/pkg/taskgroup"
)

func TestApplyPersistsDropOfGitignoredStateOnIdle(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	ignoredRel := filepath.Join(".grok", "old.md")
	keptRel := "README.md"
	ignoredAbs := filepath.Join(root, ignoredRel)
	keptAbs := filepath.Join(root, keptRel)
	content := []byte("same\n")
	for _, p := range []string{ignoredAbs, keptAbs} {
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, content, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	st, err := os.Stat(keptAbs)
	if err != nil {
		t.Fatal(err)
	}
	mode := st.Mode()

	tree := source.NewApplyTree([]source.File{
		&source.BufferFile{
			BasicFile: source.BasicFile{
				RelPathStr:    ignoredRel,
				TargetBaseDir: root,
				FileMode:      mode,
				Info:          "module:place (old.md)",
				FileType:      source.TypeStatic,
			},
			Content: content,
		},
		&source.BufferFile{
			BasicFile: source.BasicFile{
				RelPathStr:    keptRel,
				TargetBaseDir: root,
				FileMode:      mode,
				Info:          "module:place (README.md)",
				FileType:      source.TypeStatic,
			},
			Content: content,
		},
	})

	statePath := filepath.Join(root, ".workspaced", "state.json")
	store, err := deployer.NewFileStateStore(statePath, root)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.Save(&deployer.State{Files: map[string]deployer.ManagedInfo{
		ignoredAbs: {SourceInfo: "module:place (old.md)"},
		keptAbs:    {SourceInfo: "module:place (README.md)"},
	}}); err != nil {
		t.Fatal(err)
	}

	mgr, err := NewManager(Config{
		Tree:       tree,
		StateStore: store,
		Ignore:     func(target string) bool { return target == ignoredAbs },
	})
	if err != nil {
		t.Fatal(err)
	}

	g, ctx := taskgroup.New(logging.NewWriterContext(t.Output()), taskgroup.DefaultLimits())
	_ = g
	result, err := mgr.Apply(ctx, ApplyOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if result.StateDropped != 1 {
		t.Fatalf("StateDropped=%d want 1", result.StateDropped)
	}
	if result.FilesCreated+result.FilesUpdated+result.FilesDeleted != 0 {
		t.Fatalf("want idle apply, got create=%d update=%d delete=%d",
			result.FilesCreated, result.FilesUpdated, result.FilesDeleted)
	}

	loaded, err := store.Load()
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := loaded.Files[ignoredAbs]; ok {
		t.Fatal("gitignored key still in state after idle apply")
	}
	if _, ok := loaded.Files[keptAbs]; !ok {
		t.Fatal("tracked key missing after idle apply")
	}
	if _, err := os.Stat(ignoredAbs); err != nil {
		t.Fatalf("ignored file should stay on disk: %v", err)
	}
}
