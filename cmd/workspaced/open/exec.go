package open

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/lewtec/lewkit/x/cmd"
	execdriver "github.com/lucasew/workspaced/pkg/driver/exec"
	"github.com/lucasew/workspaced/pkg/taskgroup"
)

type Exec struct {
	Path []cmd.StringArg `short:"p" long:"path" help:"Prepend directories to PATH (can be specified multiple times)"`
	sep  cmd.Dash
	cmd  []cmd.StringArg
}

func (Exec) Description() string {
	return `Execute a command using the platform-appropriate exec driver

Examples:
  workspaced open exec -- git status
  workspaced open exec -- ls -la /data
  workspaced open exec -- command --with-flags
  workspaced open exec --path /custom/bin -- mycommand`
}

func (e *Exec) Run(ctx context.Context) error {
	args := cmd.Values(e.cmd)
	if len(args) == 0 {
		return fmt.Errorf("usage: workspaced open exec [flags] -- <command> [args...]")
	}

	command, err := execdriver.Run(ctx, args[0], args[1:]...)
	if err != nil {
		return fmt.Errorf("create command: %w", err)
	}

	pathDirs := cmd.Values(e.Path)
	if len(pathDirs) > 0 {
		env := os.Environ()
		if command.Env != nil {
			env = command.Env
		}

		pathModified := false
		for i, envVar := range env {
			if after, ok := strings.CutPrefix(envVar, "PATH="); ok {
				currentPath := after

				newPaths := make([]string, 0, len(pathDirs)+1)
				for _, dir := range pathDirs {
					absDir, err := filepath.Abs(dir)
					if err != nil {
						return fmt.Errorf("invalid path %q: %w", dir, err)
					}
					newPaths = append(newPaths, absDir)
				}
				newPaths = append(newPaths, currentPath)

				env[i] = "PATH=" + strings.Join(newPaths, string(os.PathListSeparator))
				pathModified = true
				break
			}
		}

		if !pathModified {
			newPaths := make([]string, 0, len(pathDirs))
			for _, dir := range pathDirs {
				absDir, err := filepath.Abs(dir)
				if err != nil {
					return fmt.Errorf("invalid path %q: %w", dir, err)
				}
				newPaths = append(newPaths, absDir)
			}
			env = append(env, "PATH="+strings.Join(newPaths, string(os.PathListSeparator)))
		}

		command.Env = env
	}

	theCmd := command
	taskgroup.MustSessionFrom(ctx).AfterWait(func() error {
		theCmd.Stdin = os.Stdin
		theCmd.Stdout = os.Stdout
		theCmd.Stderr = os.Stderr
		return theCmd.Run()
	})
	return nil
}
