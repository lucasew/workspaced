package source

import (
	"bytes"
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
)

var (
	// ErrNotSymlink is returned when a non-symlink file is asked for its link target.
	ErrNotSymlink = errors.New("not a symlink")
)

// Source represents a file/template origin.
type Source interface {
	// Name returns the unique identifier of the source.
	Name() string

	// Priority defines precedence in conflicts (higher = more priority).
	Priority() int

	// Scan discovers and returns all files from this source.
	Scan(ctx context.Context) ([]File, error)
}

// FileType indicates the kind of processing required.
type FileType int

const (
	TypeSymlink   FileType = iota // Create a direct symlink
	TypeStatic                    // Copy static file (no template)
	TypeTemplate                  // Render a simple template
	TypeMultiFile                 // Template that generates multiple files
	TypeDotD                      // .d.tmpl directory (concatenation)
)

func (t FileType) String() string {
	switch t {
	case TypeSymlink:
		return "symlink"
	case TypeStatic:
		return "static"
	case TypeTemplate:
		return "template"
	case TypeMultiFile:
		return "multifile"
	case TypeDotD:
		return "dotd"
	default:
		return "unknown"
	}
}

// File represents a file discovered or generated in the pipeline.
type File interface {
	RelPath() string
	TargetBase() string
	Mode() os.FileMode
	Reader() (io.ReadCloser, error)
	SourceInfo() string
	Type() FileType
	// LinkTarget returns the link destination if Type() == TypeSymlink.
	LinkTarget() (string, error)
}

type ScopedFile interface {
	File
	ModuleName() string
}

// BasicFile implements common File fields.
type BasicFile struct {
	RelPathStr    string
	TargetBaseDir string
	FileMode      os.FileMode
	Info          string
	FileType      FileType
	Module        string
}

func (f *BasicFile) RelPath() string    { return f.RelPathStr }
func (f *BasicFile) TargetBase() string { return f.TargetBaseDir }
func (f *BasicFile) Mode() os.FileMode  { return f.FileMode }
func (f *BasicFile) SourceInfo() string { return f.Info }
func (f *BasicFile) Type() FileType     { return f.FileType }
func (f *BasicFile) ModuleName() string { return f.Module }
func (f *BasicFile) LinkTarget() (string, error) {
	return "", ErrNotSymlink
}

// StaticFile represents a real file on disk.
type StaticFile struct {
	BasicFile
	AbsPath string
}

func (f *StaticFile) Reader() (io.ReadCloser, error) {
	return os.Open(f.AbsPath)
}

func (f *StaticFile) LinkTarget() (string, error) {
	if f.FileType != TypeSymlink {
		return "", ErrNotSymlink
	}
	return os.Readlink(f.AbsPath)
}

// BufferFile represents a file with in-memory content.
type BufferFile struct {
	BasicFile
	Content []byte
}

func (f *BufferFile) Reader() (io.ReadCloser, error) {
	return io.NopCloser(bytes.NewReader(f.Content)), nil
}

// DesiredState represents the desired state of a file.
type DesiredState struct {
	File File
}

func (d DesiredState) Target() string {
	return filepath.Join(d.File.TargetBase(), d.File.RelPath())
}

// Provider generates desired states (legacy interface, kept for compatibility).
type Provider interface {
	Name() string
	GetDesiredState(ctx context.Context) ([]DesiredState, error)
}
