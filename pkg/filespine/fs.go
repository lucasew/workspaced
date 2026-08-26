package filespine

import (
	"bytes"
	"io"
	"io/fs"
	"path"
	"sort"
	"strings"
	"time"
)

// FS is the encoded dest. Open returns the combined file.
type FS struct {
	files map[string]File
}

// NewFS builds a dest FS from merged files.
func NewFS(files map[string]File) *FS {
	cp := make(map[string]File, len(files))
	for k, v := range files {
		cp[k] = v
	}
	return &FS{files: cp}
}

func (f *FS) Open(name string) (fs.File, error) {
	if !fs.ValidPath(name) {
		return nil, &fs.PathError{Op: "open", Path: name, Err: fs.ErrInvalid}
	}
	if name == "." || f.isDir(name) {
		entries, err := f.ReadDir(name)
		if err != nil {
			return nil, err
		}
		return &dirFile{name: name, entries: entries}, nil
	}
	decl, ok := f.files[name]
	if !ok {
		return nil, &fs.PathError{Op: "open", Path: name, Err: fs.ErrNotExist}
	}
	data, err := Encode(decl)
	if err != nil {
		return nil, &fs.PathError{Op: "open", Path: name, Err: err}
	}
	return &memFile{name: name, r: bytes.NewReader(data), info: fileInfo{
		name: path.Base(name),
		size: int64(len(data)),
		mode: decl.Mode,
	}}, nil
}

func (f *FS) ReadDir(name string) ([]fs.DirEntry, error) {
	if !fs.ValidPath(name) {
		return nil, &fs.PathError{Op: "readdir", Path: name, Err: fs.ErrInvalid}
	}
	if name != "." && !f.isDir(name) {
		if _, ok := f.files[name]; ok {
			return nil, &fs.PathError{Op: "readdir", Path: name, Err: fs.ErrInvalid}
		}
		return nil, &fs.PathError{Op: "readdir", Path: name, Err: fs.ErrNotExist}
	}
	prefix := ""
	if name != "." {
		prefix = name + "/"
	}
	seen := map[string]bool{}
	entries := make([]fs.DirEntry, 0)
	for p, decl := range f.files {
		rest := p
		if prefix != "" {
			if !strings.HasPrefix(p, prefix) {
				continue
			}
			rest = p[len(prefix):]
		}
		elem, _, nested := strings.Cut(rest, "/")
		if elem == "" || seen[elem] {
			continue
		}
		seen[elem] = true
		if nested {
			entries = append(entries, dirEntry{name: elem, mode: fs.ModeDir | 0o755})
			continue
		}
		mode := decl.Mode
		if mode == 0 {
			mode = 0o644
		}
		entries = append(entries, dirEntry{name: elem, mode: mode, size: -1})
	}
	sort.Slice(entries, func(i, j int) bool {
		return entries[i].Name() < entries[j].Name()
	})
	return entries, nil
}

func (f *FS) isDir(name string) bool {
	if name == "." {
		return true
	}
	prefix := name + "/"
	for p := range f.files {
		if strings.HasPrefix(p, prefix) {
			return true
		}
	}
	return false
}

// Files returns dest paths in sorted order.
func (f *FS) Files() []File {
	out := make([]File, 0, len(f.files))
	for _, file := range f.files {
		out = append(out, file)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Path < out[j].Path })
	return out
}

type memFile struct {
	name string
	r    *bytes.Reader
	info fileInfo
}

func (f *memFile) Stat() (fs.FileInfo, error) { return f.info, nil }
func (f *memFile) Read(p []byte) (int, error) { return f.r.Read(p) }
func (f *memFile) Close() error               { return nil }

type dirFile struct {
	name    string
	entries []fs.DirEntry
	off     int
}

func (d *dirFile) Stat() (fs.FileInfo, error) {
	return fileInfo{name: path.Base(d.name), mode: fs.ModeDir | 0o755}, nil
}
func (d *dirFile) Read([]byte) (int, error) {
	return 0, &fs.PathError{Op: "read", Path: d.name, Err: errIsDir}
}
func (d *dirFile) Close() error { return nil }
func (d *dirFile) ReadDir(n int) ([]fs.DirEntry, error) {
	if d.off >= len(d.entries) {
		if n <= 0 {
			return nil, nil
		}
		return nil, io.EOF
	}
	if n <= 0 {
		out := d.entries[d.off:]
		d.off = len(d.entries)
		return out, nil
	}
	end := min(d.off+n, len(d.entries))
	out := d.entries[d.off:end]
	d.off = end
	var err error
	if d.off >= len(d.entries) {
		err = io.EOF
	}
	return out, err
}

var errIsDir = fs.ErrInvalid

type fileInfo struct {
	name string
	size int64
	mode fs.FileMode
}

func (i fileInfo) Name() string       { return i.name }
func (i fileInfo) Size() int64        { return i.size }
func (i fileInfo) Mode() fs.FileMode  { return i.mode }
func (i fileInfo) ModTime() time.Time { return time.Time{} }
func (i fileInfo) IsDir() bool        { return i.mode.IsDir() }
func (i fileInfo) Sys() any           { return nil }

type dirEntry struct {
	name string
	mode fs.FileMode
	size int64
}

func (e dirEntry) Name() string      { return e.name }
func (e dirEntry) IsDir() bool       { return e.mode.IsDir() }
func (e dirEntry) Type() fs.FileMode { return e.mode.Type() }
func (e dirEntry) Info() (fs.FileInfo, error) {
	return fileInfo{name: e.name, size: e.size, mode: e.mode}, nil
}
