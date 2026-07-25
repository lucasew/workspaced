// Package atomicfile installs files and directories via temp + rename so a
// failed or interrupted write cannot leave a truncated final path that later
// readers treat as complete.
//
// Core flow:
//
//	f, err := atomicfile.Create(path, perm)
//	if err != nil { ... }
//	defer f.Abort()
//	// write to f (io.Writer)
//	return f.Commit() // or f.CommitMode(mode)
//
// Create uses a unique temp name (safe under concurrent writers). CreateSibling
// uses the fixed path+".tmp" name when a predictable leftover is preferred.
//
// Convenience helpers Write / WriteBytes / WriteString cover the common cases.
package atomicfile

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// SiblingTemp returns path + ".tmp" (same directory as path).
// Used by CreateSibling and for cleaning known leftovers.
func SiblingTemp(path string) string {
	return path + ".tmp"
}

// File is an open temp next to Final. Write into it, then Commit (or Abort).
// After Commit or Abort, further calls are no-ops (Commit returns nil).
type File struct {
	Final string

	tmp  string
	f    *os.File
	done bool
}

// Create opens a unique temp under path's directory (os.CreateTemp) for writing.
// Concurrent writers to the same final path do not share a temp name.
// Parent directories are created. If perm is 0, 0o666 is used (subject to umask).
func Create(path string, perm os.FileMode) (*File, error) {
	if perm == 0 {
		perm = 0o666
	}
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, err
	}
	f, err := os.CreateTemp(dir, filepath.Base(path)+".tmp-*")
	if err != nil {
		return nil, err
	}
	tmp := f.Name()
	if err := f.Chmod(perm); err != nil {
		_ = f.Close()
		_ = os.Remove(tmp)
		return nil, err
	}
	return &File{Final: path, tmp: tmp, f: f}, nil
}

// CreateSibling opens a fixed sibling temp at path+".tmp" for writing.
// Prefer Create unless you need a predictable leftover name (e.g. explicit
// SiblingTemp cleanup). Concurrent writers to the same path race on that name.
// If perm is 0, 0o666 is used (same default as os.Create, subject to umask).
func CreateSibling(path string, perm os.FileMode) (*File, error) {
	if perm == 0 {
		perm = 0o666
	}
	tmp := SiblingTemp(path)
	f, err := os.OpenFile(tmp, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, perm)
	if err != nil {
		return nil, err
	}
	return &File{Final: path, tmp: tmp, f: f}, nil
}

// Name returns the temp path being written.
func (f *File) Name() string {
	if f == nil {
		return ""
	}
	return f.tmp
}

// Write implements io.Writer against the temp file.
func (f *File) Write(p []byte) (int, error) {
	if f == nil || f.f == nil || f.done {
		return 0, os.ErrClosed
	}
	return f.f.Write(p)
}

// ReadFrom implements io.ReaderFrom for efficient io.Copy into the temp.
func (f *File) ReadFrom(r io.Reader) (int64, error) {
	if f == nil || f.f == nil || f.done {
		return 0, os.ErrClosed
	}
	return io.Copy(f.f, r)
}

// Abort closes and removes the temp. Safe after Commit (no-op).
func (f *File) Abort() {
	if f == nil || f.done {
		return
	}
	f.done = true
	if f.f != nil {
		_ = f.f.Close()
		f.f = nil
	}
	if f.tmp != "" {
		_ = os.Remove(f.tmp)
	}
}

// Commit closes the temp and renames it onto Final.
// On failure the temp is removed and an existing Final is left untouched.
func (f *File) Commit() error {
	return f.commit(0)
}

// CommitMode is Commit plus chmod(Final, mode) after a successful rename.
// Use when the desired mode must survive umask (e.g. 0o755 binaries).
func (f *File) CommitMode(mode os.FileMode) error {
	return f.commit(mode)
}

func (f *File) commit(mode os.FileMode) error {
	if f == nil {
		return os.ErrInvalid
	}
	if f.done {
		return nil
	}
	if f.f != nil {
		err := f.f.Close()
		f.f = nil
		if err != nil {
			f.Abort()
			return err
		}
	}
	// Install owns tmp removal on rename failure.
	err := Install(f.tmp, f.Final, mode)
	f.done = true
	f.tmp = ""
	return err
}

// Install renames tmp onto dest. On rename failure tmp is removed.
// If mode is non-zero, dest is chmod'd after a successful rename.
func Install(tmp, dest string, mode os.FileMode) error {
	if err := os.Rename(tmp, dest); err != nil {
		_ = os.Remove(tmp)
		return err
	}
	if mode != 0 {
		if err := os.Chmod(dest, mode); err != nil {
			return err
		}
	}
	return nil
}

// Write streams r into path via Create + Commit.
// If perm is 0, 0o666 is used. After rename, perm is re-applied when non-zero
// so executable bits survive umask.
func Write(path string, r io.Reader, perm os.FileMode) error {
	f, err := Create(path, perm)
	if err != nil {
		return err
	}
	defer f.Abort()
	if _, err := io.Copy(f, r); err != nil {
		return err
	}
	if perm != 0 {
		return f.CommitMode(perm)
	}
	return f.Commit()
}

// WriteBytes writes b to path via temp + rename.
func WriteBytes(path string, b []byte, perm os.FileMode) error {
	return Write(path, bytes.NewReader(b), perm)
}

// WriteString writes s to path via temp + rename.
func WriteString(path string, s string, perm os.FileMode) error {
	return Write(path, strings.NewReader(s), perm)
}

// ReplaceDir moves tmpDir into place at dest, replacing any existing dest.
// On failure to rename tmpDir into place, a previous dest is restored from
// dest+".old" when possible. Parent of dest is created if missing.
func ReplaceDir(dest, tmpDir string) error {
	if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
		return err
	}
	old := dest + ".old"
	_ = os.RemoveAll(old)
	if _, err := os.Stat(dest); err == nil {
		if err := os.Rename(dest, old); err != nil {
			if err := os.RemoveAll(dest); err != nil {
				return err
			}
		}
	}
	if err := os.Rename(tmpDir, dest); err != nil {
		_ = os.Rename(old, dest)
		return err
	}
	_ = os.RemoveAll(old)
	return nil
}
