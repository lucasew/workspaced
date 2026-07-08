package source

import (
	"io"
	"os"
	"path/filepath"
	"testing"
)

// trackingCloser records whether Close was called.
type trackingCloser struct {
	io.ReadCloser
	closed *bool
}

func (t *trackingCloser) Close() error {
	*t.closed = true
	return t.ReadCloser.Close()
}

func TestConcatenatedFileReaderCloseClosesComponents(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	paths := []string{
		filepath.Join(dir, "a.txt"),
		filepath.Join(dir, "b.txt"),
	}
	for i, p := range paths {
		content := []byte{'A' + byte(i)}
		if err := os.WriteFile(p, content, 0o644); err != nil {
			t.Fatalf("write %s: %v", p, err)
		}
	}

	closed := make([]bool, len(paths))
	components := make([]File, len(paths))
	for i, p := range paths {
		sf := &StaticFile{
			BasicFile: BasicFile{
				RelPathStr:    filepath.Base(p),
				TargetBaseDir: dir,
				FileMode:      0o644,
				FileType:      TypeStatic,
			},
			AbsPath: p,
		}
		// Wrap via a custom File that tracks Close through Reader.
		components[i] = &trackingFile{StaticFile: sf, closed: &closed[i]}
	}

	cf := &ConcatenatedFile{
		BasicFile: BasicFile{
			RelPathStr:    "out",
			TargetBaseDir: dir,
			FileMode:      0o644,
			FileType:      TypeDotD,
		},
		Components: components,
	}

	r, err := cf.Reader()
	if err != nil {
		t.Fatalf("Reader: %v", err)
	}
	got, err := io.ReadAll(r)
	if err != nil {
		t.Fatalf("ReadAll: %v", err)
	}
	if string(got) != "A\nB" {
		t.Fatalf("content = %q, want %q", got, "A\nB")
	}
	// Partial read path already finished; Close must still close components.
	if err := r.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	for i, c := range closed {
		if !c {
			t.Errorf("component %d not closed", i)
		}
	}
}

func TestConcatenatedFileCloseWithoutFullRead(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	p := filepath.Join(dir, "big.txt")
	if err := os.WriteFile(p, []byte("hello world"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	var closed bool
	cf := &ConcatenatedFile{
		BasicFile: BasicFile{RelPathStr: "out", TargetBaseDir: dir, FileMode: 0o644, FileType: TypeDotD},
		Components: []File{
			&trackingFile{
				StaticFile: &StaticFile{
					BasicFile: BasicFile{RelPathStr: "big.txt", TargetBaseDir: dir, FileMode: 0o644, FileType: TypeStatic},
					AbsPath:   p,
				},
				closed: &closed,
			},
		},
	}
	r, err := cf.Reader()
	if err != nil {
		t.Fatalf("Reader: %v", err)
	}
	buf := make([]byte, 1)
	if _, err := r.Read(buf); err != nil {
		t.Fatalf("Read: %v", err)
	}
	if err := r.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	if !closed {
		t.Fatal("component not closed after partial read + Close")
	}
}

// trackingFile is a StaticFile whose Reader returns a closer that records Close.
type trackingFile struct {
	*StaticFile
	closed *bool
}

func (f *trackingFile) Reader() (io.ReadCloser, error) {
	r, err := f.StaticFile.Reader()
	if err != nil {
		return nil, err
	}
	return &trackingCloser{ReadCloser: r, closed: f.closed}, nil
}
