package checks

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"

	"github.com/lucasew/workspaced/internal/tool"
	execdriver "github.com/lucasew/workspaced/pkg/driver/exec"
)

// ResolveCmd ensures lazy tools from needs and returns argv + optional env extras.
func ResolveCmd(ctx context.Context, root string, t Tool) (argv []string, envExtra []string, err error) {
	if len(t.Cmd) == 0 {
		return nil, nil, fmt.Errorf("%w: tool %q", ErrEmptyCmd, t.Name)
	}
	argv, envExtra, err = tool.ResolveNeedsCmd(ctx, root, t.Cmd, t.Needs)
	if err != nil {
		return nil, nil, fmt.Errorf("tool %q: %w", t.Name, err)
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
