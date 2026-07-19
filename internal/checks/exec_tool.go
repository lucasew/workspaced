package checks

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"workspaced/internal/tool"
	execdriver "workspaced/pkg/driver/exec"
)

// ResolveCmd ensures lazy tools from needs and returns argv + optional env extras.
func ResolveCmd(ctx context.Context, root string, t Tool) (argv []string, envExtra []string, err error) {
	argv = append([]string(nil), t.Cmd...)
	if len(argv) == 0 {
		return nil, nil, fmt.Errorf("tool %q: empty cmd", t.Name)
	}

	var pathDirs []string
	for name, on := range t.Needs {
		if !on {
			continue
		}
		binName := filepath.Base(argv[0])
		binPath, rerr := tool.ResolveLazyToolAt(ctx, root, name, binName)
		if rerr != nil {
			binPath, rerr = tool.ResolveLazyToolAt(ctx, root, name, name)
		}
		if rerr != nil {
			return nil, nil, fmt.Errorf("tool %q: ensure lazy tool %q: %w", t.Name, name, rerr)
		}
		dir := filepath.Dir(binPath)
		pathDirs = append(pathDirs, dir)
		if filepath.Base(argv[0]) == filepath.Base(binPath) || argv[0] == name {
			argv[0] = binPath
		}
	}

	if len(pathDirs) > 0 {
		path := strings.Join(pathDirs, string(os.PathListSeparator))
		if existing := os.Getenv("PATH"); existing != "" {
			path = path + string(os.PathListSeparator) + existing
		}
		envExtra = append(envExtra, "PATH="+path)
	}
	return argv, envExtra, nil
}

// BuildCmd constructs an exec.Cmd for the tool (no run).
// If argsFromGlobs and detect yielded a glob, matched files are appended.
func BuildCmd(ctx context.Context, root string, t Tool, detect DetectResult) (*exec.Cmd, error) {
	argv, envExtra, err := ResolveCmd(ctx, root, t)
	if err != nil {
		return nil, err
	}
	if t.ArgsFromGlobs && detect.Glob != "" {
		files, err := CollectGlob(root, detect.Glob)
		if err != nil {
			return nil, fmt.Errorf("tool %q: expand globs: %w", t.Name, err)
		}
		if len(files) == 0 {
			return nil, fmt.Errorf("tool %q: args_from_globs but no files matched %q", t.Name, detect.Glob)
		}
		argv = append(argv, files...)
	}

	cmd, err := execdriver.Run(ctx, argv[0], argv[1:]...)
	if err != nil {
		return nil, err
	}
	cmd.Dir = root
	if len(envExtra) > 0 {
		cmd.Env = append(os.Environ(), envExtra...)
	}
	return cmd, nil
}

// RunCapture runs the tool, capturing stdout/stderr.
// Returns stdout even when the process exits non-zero (common for linters with findings).
func RunCapture(cmd *exec.Cmd) (stdout, stderr []byte, runErr error) {
	var outBuf, errBuf bytes.Buffer
	cmd.Stdout = &outBuf
	cmd.Stderr = &errBuf
	runErr = cmd.Run()
	return outBuf.Bytes(), errBuf.Bytes(), runErr
}
