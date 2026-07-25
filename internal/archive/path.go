// Package archive provides path-jailing and member write helpers for unpacking
// archives under a destination directory (tool install, module tarballs).
package archive

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// PathWithinDest reports whether target is dest or a path under dest after Clean.
func PathWithinDest(dest, target string) bool {
	cleanDest := filepath.Clean(dest)
	cleanTarget := filepath.Clean(target)

	rel, err := filepath.Rel(cleanDest, cleanTarget)
	if err != nil {
		return false
	}
	if rel == "." {
		return true
	}
	return rel != ".." && !strings.HasPrefix(rel, ".."+string(os.PathSeparator))
}

// JoinWithin joins name under destDir and returns an error if the result would
// escape destDir (zip/tar slip). Absolute name segments are rejected.
func JoinWithin(destDir, name string) (string, error) {
	if name == "" {
		return "", fmt.Errorf("illegal file path: empty name")
	}
	// filepath.Join discards prior segments after an absolute element on some OSes.
	if filepath.IsAbs(name) {
		return "", fmt.Errorf("illegal file path: %s", name)
	}
	target := filepath.Join(destDir, name)
	if !PathWithinDest(destDir, target) {
		return "", fmt.Errorf("illegal file path: %s", name)
	}
	return target, nil
}

// ResolveWithin evaluates symlinks in target's parent and ensures the resolved
// path stays under destDir. Call after MkdirAll of the parent.
func ResolveWithin(destDir, target string) (string, error) {
	parent := filepath.Dir(target)
	realParent, err := filepath.EvalSymlinks(parent)
	if err != nil {
		return "", err
	}
	if !PathWithinDest(destDir, realParent) {
		return "", fmt.Errorf("illegal file path: %s", target)
	}
	return filepath.Join(realParent, filepath.Base(target)), nil
}

// SymlinkTargetWithin reports whether linkname, resolved from linkPath's parent,
// stays under destDir (including through existing parent symlinks).
func SymlinkTargetWithin(destDir, linkPath, linkname string) bool {
	if linkname == "" {
		return false
	}
	realParent, err := filepath.EvalSymlinks(filepath.Dir(linkPath))
	if err != nil {
		return false
	}
	if !PathWithinDest(destDir, realParent) {
		return false
	}
	var resolved string
	if filepath.IsAbs(linkname) {
		resolved = filepath.Clean(linkname)
	} else {
		resolved = filepath.Join(realParent, linkname)
	}
	return PathWithinDest(destDir, resolved)
}

// WriteMember creates path with mode, copies r into it, and removes path on any
// failure after create. Used for archive entry extraction (not atomic rename —
// the destination is usually a staging dir that is swapped later).
func WriteMember(path string, mode os.FileMode, r io.Reader) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	if mode == 0 {
		mode = 0o644
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, mode)
	if err != nil {
		return err
	}
	_, copyErr := io.Copy(f, r)
	closeErr := f.Close()
	if copyErr != nil {
		_ = os.Remove(path)
		return copyErr
	}
	if closeErr != nil {
		_ = os.Remove(path)
		return closeErr
	}
	// Re-apply executable bits when OpenFile's mode was masked by umask.
	if mode&0o111 != 0 {
		if err := os.Chmod(path, mode); err != nil {
			_ = os.Remove(path)
			return fmt.Errorf("set permissions: %w", err)
		}
	}
	return nil
}
