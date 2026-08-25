package source

import (
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/lucasew/workspaced/internal/configcue"
	"github.com/lucasew/workspaced/pkg/logging"
)

func TestFileSpineLowersDotD(t *testing.T) {
	t.Parallel()
	ctx := logging.NewWriterContext(t.Output())
	home := t.TempDir()
	p := NewFileSpinePlugin(&configcue.Config{}, home)
	in := []File{
		&BufferFile{
			BasicFile: BasicFile{RelPathStr: ".bashrc.d.tmpl/20-b.sh", TargetBaseDir: home, FileMode: 0o644, FileType: TypeStatic},
			Content:   []byte("b"),
		},
		&BufferFile{
			BasicFile: BasicFile{RelPathStr: ".bashrc.d.tmpl/10-a.sh", TargetBaseDir: home, FileMode: 0o644, FileType: TypeStatic},
			Content:   []byte("a"),
		},
	}
	out, err := p.Process(ctx, in)
	if err != nil {
		t.Fatal(err)
	}
	if len(out) != 1 {
		t.Fatalf("len=%d want 1", len(out))
	}
	if out[0].RelPath() != ".bashrc" {
		t.Fatalf("rel=%q", out[0].RelPath())
	}
	r, err := out[0].Reader()
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	got, err := io.ReadAll(r)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "a\nb" {
		t.Fatalf("content=%q", got)
	}
}

func TestFileSpineMergesCueLines(t *testing.T) {
	t.Parallel()
	ctx := logging.NewWriterContext(t.Output())
	home := t.TempDir()
	cuePath := filepath.Join(t.TempDir(), "workspaced.cue")
	src := `package workspaced
workspaced: {
	file: ".bashrc": {
		type: "lines"
		values: {"00-cue": "from-cue"}
	}
}
`
	if err := os.WriteFile(cuePath, []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg, err := configcue.LoadFiles(ctx, []string{cuePath})
	if err != nil {
		t.Fatal(err)
	}
	p := NewFileSpinePlugin(cfg, home)
	in := []File{
		&BufferFile{
			BasicFile: BasicFile{RelPathStr: ".bashrc.d.tmpl/10-mod.sh", TargetBaseDir: home, FileMode: 0o644},
			Content:   []byte("from-mod"),
		},
	}
	out, err := p.Process(ctx, in)
	if err != nil {
		t.Fatal(err)
	}
	if len(out) != 1 {
		t.Fatalf("len=%d", len(out))
	}
	r, err := out[0].Reader()
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	got, err := io.ReadAll(r)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "from-cue\nfrom-mod" {
		t.Fatalf("content=%q", got)
	}
}

func TestFileSpineTypeConflict(t *testing.T) {
	t.Parallel()
	ctx := logging.NewWriterContext(t.Output())
	home := t.TempDir()
	dir := t.TempDir()
	cuePath := filepath.Join(dir, "workspaced.cue")
	if err := os.WriteFile(cuePath, []byte(`package workspaced
workspaced: file: "x": {type: "lines", values: {a: "1"}}
`), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg, err := configcue.LoadFiles(ctx, []string{cuePath})
	if err != nil {
		t.Fatal(err)
	}
	p := NewFileSpinePlugin(cfg, home)
	_, err = p.Process(ctx, []File{
		&BufferFile{
			BasicFile: BasicFile{RelPathStr: "x", TargetBaseDir: home, FileMode: 0o644},
			Content:   []byte("text"),
		},
	})
	if err == nil {
		t.Fatal("expected type conflict")
	}
}

func TestFileSpineStaticRef(t *testing.T) {
	t.Parallel()
	ctx := logging.NewWriterContext(t.Output())
	home := t.TempDir()
	src := filepath.Join(t.TempDir(), "gitconfig")
	if err := os.WriteFile(src, []byte("[user]\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	p := NewFileSpinePlugin(&configcue.Config{}, home)
	out, err := p.Process(ctx, []File{
		&StaticFile{
			BasicFile: BasicFile{RelPathStr: ".gitconfig", TargetBaseDir: home, FileMode: 0o644, FileType: TypeStatic},
			AbsPath:   src,
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(out) != 1 {
		t.Fatalf("len=%d", len(out))
	}
	sf, ok := out[0].(*StaticFile)
	if !ok {
		t.Fatalf("type %T", out[0])
	}
	if sf.AbsPath != src {
		t.Fatalf("abs=%q", sf.AbsPath)
	}
}
