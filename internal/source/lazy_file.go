package source

import (
	"bytes"
	"context"
	"errors"
	"io"

	"workspaced/internal/template"
	"workspaced/pkg/logging"
)

// TemplateFile represents a lazily-rendered template file.
type TemplateFile struct {
	BasicFile
	SourceFile File
	Engine     *template.Engine
	Data       any
	Context    context.Context
}

func (f *TemplateFile) Reader() (io.ReadCloser, error) {
	srcReader, err := f.SourceFile.Reader()
	if err != nil {
		return nil, err
	}
	defer logging.Close(f.Context, srcReader)

	srcContent, err := io.ReadAll(srcReader)
	if err != nil {
		return nil, err
	}

	rendered, err := f.Engine.Render(f.Context, string(srcContent), f.Data)
	if err != nil {
		return nil, err
	}

	return io.NopCloser(bytes.NewReader(rendered)), nil
}

// ConcatenatedFile represents multiple files concatenated together (DotD).
type ConcatenatedFile struct {
	BasicFile
	Components []File
}

// multiReadCloser chains readers and closes every component on Close.
// io.MultiReader alone does not close underlying ReadClosers; NopCloser
// on top of it made Close a no-op and leaked open files.
type multiReadCloser struct {
	r       io.Reader
	closers []io.Closer
}

func (m *multiReadCloser) Read(p []byte) (int, error) {
	return m.r.Read(p)
}

func (m *multiReadCloser) Close() error {
	var errs []error
	for _, c := range m.closers {
		if err := c.Close(); err != nil {
			errs = append(errs, err)
		}
	}
	return errors.Join(errs...)
}

func (f *ConcatenatedFile) Reader() (io.ReadCloser, error) {
	readers := make([]io.Reader, 0, len(f.Components)*2)
	closers := make([]io.Closer, 0, len(f.Components))
	for i, c := range f.Components {
		r, err := c.Reader()
		if err != nil {
			for _, cl := range closers {
				_ = cl.Close()
			}
			return nil, err
		}
		closers = append(closers, r)
		readers = append(readers, r)
		if i < len(f.Components)-1 {
			readers = append(readers, bytes.NewReader([]byte("\n")))
		}
	}
	return &multiReadCloser{r: io.MultiReader(readers...), closers: closers}, nil
}
