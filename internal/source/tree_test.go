package source

import (
	"io/fs"
	"os"
	"path/filepath"
	"testing"

	"github.com/lucasew/workspaced/internal/configcue"
	_ "github.com/lucasew/workspaced/pkg/driver/env/native"
	"github.com/lucasew/workspaced/pkg/logging"
)

func TestBuildTreeRendersTemplateAndStatic(t *testing.T) {
	t.Parallel()
	ctx := logging.NewWriterContext(t.Output())
	src := t.TempDir()
	dest := t.TempDir()
	if err := os.WriteFile(filepath.Join(src, "hello.txt.tmpl"), []byte("hi {{ .runtime.goos }}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(src, "plain.txt"), []byte("static\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	scanner, err := NewScannerPlugin(ScannerConfig{Name: "src", BaseDir: src, TargetBase: dest})
	if err != nil {
		t.Fatal(err)
	}
	tree, err := BuildTree(ctx, &configcue.Config{}, dest, scanner)
	if err != nil {
		t.Fatal(err)
	}
	if tree.Dest() == nil {
		t.Fatal("Dest is nil")
	}
	got, err := fs.ReadFile(tree.Dest(), "plain.txt")
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "static\n" {
		t.Fatalf("plain=%q", got)
	}
	hello, err := fs.ReadFile(tree.Dest(), "hello.txt")
	if err != nil {
		t.Fatal(err)
	}
	if len(hello) == 0 {
		t.Fatal("hello.txt empty")
	}
	files := tree.Files()
	if len(files) != 2 {
		t.Fatalf("files=%d want 2", len(files))
	}
}

func TestBuildTreeMergesCueLines(t *testing.T) {
	t.Parallel()
	ctx := logging.NewWriterContext(t.Output())
	src := t.TempDir()
	dest := t.TempDir()
	if err := os.MkdirAll(filepath.Join(src, ".bashrc.d.tmpl"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(src, ".bashrc.d.tmpl", "10-mod.sh"), []byte("from-mod"), 0o644); err != nil {
		t.Fatal(err)
	}
	cuePath := filepath.Join(t.TempDir(), "workspaced.cue")
	if err := os.WriteFile(cuePath, []byte(`package workspaced
workspaced: {
	file: ".bashrc": {
		type: "lines"
		values: {"00-cue": "from-cue"}
	}
}
`), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg, err := configcue.LoadFiles(ctx, []string{cuePath})
	if err != nil {
		t.Fatal(err)
	}
	scanner, err := NewScannerPlugin(ScannerConfig{Name: "src", BaseDir: src, TargetBase: dest})
	if err != nil {
		t.Fatal(err)
	}
	tree, err := BuildTree(ctx, cfg, dest, scanner)
	if err != nil {
		t.Fatal(err)
	}
	got, err := fs.ReadFile(tree.Dest(), ".bashrc")
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "from-cue\nfrom-mod" {
		t.Fatalf("content=%q", got)
	}
}

func TestTreeFromFilesHasNoDest(t *testing.T) {
	t.Parallel()
	tree := TreeFromFiles([]File{&BufferFile{
		BasicFile: BasicFile{RelPathStr: "a", TargetBaseDir: "/tmp"},
		Content:   []byte("x"),
	}})
	if tree.Dest() != nil {
		t.Fatal("expected nil Dest")
	}
	if len(tree.Files()) != 1 {
		t.Fatalf("files=%d", len(tree.Files()))
	}
}
