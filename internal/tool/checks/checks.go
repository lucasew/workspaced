// Package checks defines declarative validations for a tool install directory.
//
// Tools may implement InstallChecker to declare post-install expectations
// (file existence, executability). Manager and tests call Run after install
// (and when reusing an existing tree) to enforce them.
package checks

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"workspaced/internal/tool/backend"
)

// ErrFailed is returned when one or more install checks do not pass.
var ErrFailed = errors.New("install check failed")

// ErrBinaryNotFound is returned when Binary cannot locate cmdName under destDir.
var ErrBinaryNotFound = errors.New("binary not found in install directory")

// Check validates one expectation about an installed tool tree.
type Check interface {
	Name() string
	Check(ctx context.Context, destDir string) error
}

// InstallChecker is an optional extension on backend.Tool.
// Tools that implement it declare checks run against their install directory.
type InstallChecker interface {
	InstallChecks() []Check
}

// Run type-asserts t as InstallChecker. It is a no-op if t does not implement
// it or returns no checks. Otherwise every check runs and errors are joined.
func Run(ctx context.Context, destDir string, t backend.Tool) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	ic, ok := t.(InstallChecker)
	if !ok {
		return nil
	}
	list := ic.InstallChecks()
	if len(list) == 0 {
		return nil
	}
	var errs []error
	for _, c := range list {
		if err := ctx.Err(); err != nil {
			errs = append(errs, err)
			break
		}
		if err := c.Check(ctx, destDir); err != nil {
			errs = append(errs, fmt.Errorf("%s: %w", c.Name(), err))
		}
	}
	if len(errs) == 0 {
		return nil
	}
	return fmt.Errorf("%w: %w", ErrFailed, errors.Join(errs...))
}

// FileExists checks that destDir/relPath exists as a regular file.
func FileExists(relPath string) Check {
	return pathCheck{
		name:    "exists:" + relPath,
		relPath: relPath,
		fn: func(_ context.Context, path string, info fs.FileInfo) error {
			if info.IsDir() {
				return fmt.Errorf("%s is a directory, want a file", path)
			}
			return nil
		},
	}
}

// Executable checks that destDir/relPath exists and is executable.
// On Windows, existence as a non-directory file is sufficient.
func Executable(relPath string) Check {
	return pathCheck{
		name:    "executable:" + relPath,
		relPath: relPath,
		fn: func(_ context.Context, path string, info fs.FileInfo) error {
			if info.IsDir() {
				return fmt.Errorf("%s is a directory, want an executable file", path)
			}
			if runtime.GOOS == "windows" {
				return nil
			}
			if info.Mode()&0o111 == 0 {
				return fmt.Errorf("%s is not executable", path)
			}
			return nil
		},
	}
}

// Binary locates cmdName under destDir using the standard candidate layout,
// then runs FileExists and Executable on the located relative path.
func Binary(cmdName string) Check {
	return binaryCheck{cmdName: cmdName}
}

type pathCheck struct {
	name    string
	relPath string
	fn      func(ctx context.Context, path string, info fs.FileInfo) error
}

func (c pathCheck) Name() string { return c.name }

func (c pathCheck) Check(ctx context.Context, destDir string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	path, err := safeJoin(destDir, c.relPath)
	if err != nil {
		return err
	}
	info, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("%s: %w", path, fs.ErrNotExist)
		}
		return err
	}
	return c.fn(ctx, path, info)
}

type binaryCheck struct {
	cmdName string
}

func (c binaryCheck) Name() string { return "binary:" + c.cmdName }

func (c binaryCheck) Check(ctx context.Context, destDir string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	path := FindBinary(destDir, c.cmdName)
	if path == "" {
		return fmt.Errorf("%w: %q in %s", ErrBinaryNotFound, c.cmdName, destDir)
	}
	rel, err := filepath.Rel(destDir, path)
	if err != nil {
		return err
	}
	if err := FileExists(rel).Check(ctx, destDir); err != nil {
		return err
	}
	return Executable(rel).Check(ctx, destDir)
}

func safeJoin(destDir, relPath string) (string, error) {
	destAbs, err := filepath.Abs(destDir)
	if err != nil {
		return "", err
	}
	cleanRel := filepath.Clean("/" + filepath.ToSlash(relPath))
	cleanRel = strings.TrimPrefix(cleanRel, "/")
	if cleanRel == "" || cleanRel == "." {
		return "", fmt.Errorf("empty relative path")
	}
	joined := filepath.Join(destAbs, filepath.FromSlash(cleanRel))
	joinedAbs, err := filepath.Abs(joined)
	if err != nil {
		return "", err
	}
	sep := string(os.PathSeparator)
	if joinedAbs != destAbs && !strings.HasPrefix(joinedAbs, destAbs+sep) {
		return "", fmt.Errorf("path %q escapes install directory", relPath)
	}
	return joinedAbs, nil
}
