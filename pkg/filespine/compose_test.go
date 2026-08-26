package filespine

import (
	"context"
	"io/fs"
	"os"
	"path/filepath"
	"testing"
)

type mapProvider struct {
	name  string
	patch Patch
}

func (p mapProvider) Name() string { return p.name }
func (p mapProvider) Provide(context.Context) (Patch, error) {
	return p.patch, nil
}

func TestComposeCueAndSlots(t *testing.T) {
	t.Parallel()
	dest, err := Compose(t.Context(),
		mapProvider{name: "cue", patch: Patch{Files: map[string]File{
			".bashrc": {Path: ".bashrc", Type: TypeLines, Values: map[string]Slot{
				"00-cue": {Kind: KindText, Text: "from-cue"},
			}},
		}}},
		mapProvider{name: "static", patch: Patch{Slots: []Contribution{{
			Path: ".gitconfig", Type: TypeText, Key: "content",
			Slot: Slot{Kind: KindText, Text: "[user]\n"},
		}}}},
		mapProvider{name: "templates", patch: Patch{Slots: []Contribution{{
			Path: ".bashrc", Type: TypeLines, Key: "10-mod",
			Slot: Slot{Kind: KindText, Text: "from-mod"},
		}}}},
	)
	if err != nil {
		t.Fatal(err)
	}
	got, err := fs.ReadFile(dest, ".bashrc")
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "from-cue\nfrom-mod" {
		t.Fatalf("bashrc=%q", got)
	}
	cfg, err := fs.ReadFile(dest, ".gitconfig")
	if err != nil {
		t.Fatal(err)
	}
	if string(cfg) != "[user]\n" {
		t.Fatalf("gitconfig=%q", cfg)
	}
}

func TestStaticDirSkipsTemplates(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "plain.txt"), []byte("ok\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "x.tmpl"), []byte("no"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(root, ".bashrc.d.tmpl"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, ".bashrc.d.tmpl", "10.sh"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	dest, err := Compose(t.Context(), StaticDir{Root: root})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := fs.ReadFile(dest, "plain.txt"); err != nil {
		t.Fatal(err)
	}
	if _, err := dest.Open("x"); err == nil {
		t.Fatal("template should not appear")
	}
	if _, err := dest.Open(".bashrc"); err == nil {
		t.Fatal("dotd should not appear")
	}
}
