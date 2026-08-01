package miseutil

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/lucasew/workspaced/internal/atomicfile"
	"github.com/lucasew/workspaced/internal/tool"
	envdriver "github.com/lucasew/workspaced/pkg/driver/env"
	execdriver "github.com/lucasew/workspaced/pkg/driver/exec"
	"github.com/lucasew/workspaced/pkg/driver/shim/bash"
	"github.com/lucasew/workspaced/pkg/logging"
)

var (
	// ErrBinaryNotFound is returned when a binary is not found in a mise tool install tree.
	ErrBinaryNotFound = errors.New("binary not found")
)

// Ensure returns a path to the mise CLI via the standard home lazy tool route
// (lazy_tools.mise → registry:mise). Not a separate install system: same path
// as `workspaced open lazy --home mise`.
//
// Falls back to a direct registry:mise ensure when home config/lock cannot be
// used (bootstrap, missing dotfiles root, read-only lock).
func Ensure(ctx context.Context) (string, error) {
	if path, err := tool.ResolveHomeLazyTool(ctx, "mise", "mise"); err == nil {
		return path, nil
	} else {
		logging.GetLogger(ctx).Debug("home lazy mise resolve failed; falling back to registry ensure", "error", err)
	}
	mgr, err := tool.NewManager()
	if err != nil {
		return "", err
	}
	return mgr.EnsureInstalled(ctx, "registry:mise", "mise")
}

// Output runs the mise CLI with args and returns combined stdout.
// Used by the mise: package backend (not for installing mise itself).
func Output(ctx context.Context, args ...string) ([]byte, error) {
	misePath, err := Ensure(ctx)
	if err != nil {
		return nil, err
	}
	cmd, err := execdriver.Run(ctx, misePath, args...)
	if err != nil {
		return nil, err
	}
	return cmd.Output()
}

// Run runs the mise CLI with args, wiring stdio to the process.
func Run(ctx context.Context, args ...string) error {
	misePath, err := Ensure(ctx)
	if err != nil {
		return err
	}
	cmd, err := execdriver.Run(ctx, misePath, args...)
	if err != nil {
		return err
	}
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin
	return cmd.Run()
}

// Latest resolves the latest version of a mise package spec (e.g. "node").
func Latest(ctx context.Context, spec string) (string, error) {
	return trimmedOutput(ctx, "latest", spec)
}

// Where returns the install root for a mise package spec.
func Where(ctx context.Context, toolSpec string) (string, error) {
	return trimmedOutput(ctx, "where", toolSpec)
}

func trimmedOutput(ctx context.Context, args ...string) (string, error) {
	out, err := Output(ctx, args...)
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}

// ResolveBinPath finds binName under the install root of a mise package spec.
func ResolveBinPath(ctx context.Context, binName, toolSpec string) (string, error) {
	root, err := Where(ctx, toolSpec)
	if err != nil {
		return "", err
	}

	if binPath := tool.FindBinary(root, binName); binPath != "" {
		return binPath, nil
	}

	return "", fmt.Errorf("%w: %q under %s", ErrBinaryNotFound, binName, root)
}

// EnsureLocalBinWrapper writes ~/.local/bin/mise so PATH users re-enter the
// standard lazy route (open lazy --home mise). Integration only — not a
// separate install path for the binary.
//
// workspacedBin is the absolute path to the workspaced binary; when empty,
// the default under the user data dir is used.
func EnsureLocalBinWrapper(ctx context.Context, workspacedBin string) error {
	logger := logging.GetLogger(ctx)
	home, err := envdriver.ResolveHomeDir()
	if err != nil {
		return fmt.Errorf("get home directory: %w", err)
	}

	if strings.TrimSpace(workspacedBin) == "" {
		dataDir, derr := envdriver.GetUserDataDir(ctx)
		if derr != nil {
			dataDir = filepath.Join(home, ".local", "share", "workspaced")
		}
		workspacedBin = filepath.Join(dataDir, "bin", "workspaced")
	}

	wrapperDir := filepath.Join(home, ".local", "bin")
	wrapperPath := filepath.Join(wrapperDir, "mise")
	shell := bash.GetShell(ctx)
	// Same argv shape as modules/mise and open lazy --home.
	expectedContent := fmt.Sprintf(
		"#!%s\nexec -a \"$0\" %s open lazy --home --bin mise mise -- \"$@\"\n",
		shell, workspacedBin,
	)

	if content, err := os.ReadFile(wrapperPath); err == nil && string(content) == expectedContent {
		return nil
	}

	if err := os.MkdirAll(wrapperDir, 0o755); err != nil {
		return fmt.Errorf("create wrapper directory: %w", err)
	}
	if err := atomicfile.WriteString(wrapperPath, expectedContent, 0o755); err != nil {
		return fmt.Errorf("write mise wrapper: %w", err)
	}

	logger.Info("created mise wrapper", "path", wrapperPath, "workspaced", workspacedBin)
	return nil
}
