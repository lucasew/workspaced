package miseutil

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/lucasew/workspaced/internal/cmdctx"
	"github.com/lucasew/workspaced/internal/tool"
	"github.com/lucasew/workspaced/pkg/driver"
	envdriver "github.com/lucasew/workspaced/pkg/driver/env"
	execdriver "github.com/lucasew/workspaced/pkg/driver/exec"
	"github.com/lucasew/workspaced/pkg/driver/httpclient"
	"github.com/lucasew/workspaced/pkg/driver/shim/bash"
	"github.com/lucasew/workspaced/pkg/logging"
)

const installerURL = "https://mise.run"

var (
	// ErrMiseInstallPathUnknown is returned when the mise install path cannot be determined.
	ErrMiseInstallPathUnknown = errors.New("could not determine mise install path")
	// ErrBinaryNotFound is returned when a binary is not found in the mise install tree.
	ErrBinaryNotFound = errors.New("binary not found")
	// ErrMiseInstallFailed is returned when mise installation fails.
	ErrMiseInstallFailed = errors.New("mise installation failed")
	// ErrHTTPDownloadFailed is returned when downloading the official installer fails.
	ErrHTTPDownloadFailed = errors.New("HTTP download failed")
)

func GetPath() string {
	if path := os.Getenv("MISE_INSTALL_PATH"); path != "" {
		return path
	}

	home, err := envdriver.ResolveHomeDir()
	if err != nil {
		return ""
	}

	return filepath.Join(home, ".local", "share", "workspaced", "bin", "mise")
}

func Ensure(ctx context.Context) (string, error) {
	misePath := GetPath()
	if misePath == "" {
		return "", ErrMiseInstallPathUnknown
	}

	logger := logging.GetLogger(ctx)
	noCache := cmdctx.IsNoCache(ctx)
	if _, err := os.Stat(misePath); err == nil && !noCache {
		return misePath, nil
	}
	if noCache && cmdctx.IsDryRun(ctx) {
		if _, err := os.Stat(misePath); err == nil {
			logger.Debug("no-cache: would reinstall mise (dry-run)", "path", misePath)
			return misePath, nil
		}
	}
	if noCache {
		logger.Debug("no-cache: reinstalling mise", "path", misePath)
	} else {
		logger.Info("mise not found, installing", "path", misePath)
	}
	if err := Install(ctx, misePath); err != nil {
		return "", err
	}

	return misePath, nil
}

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

func Latest(ctx context.Context, spec string) (string, error) {
	out, err := Output(ctx, "latest", spec)
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}

func Where(ctx context.Context, toolSpec string) (string, error) {
	out, err := Output(ctx, "where", toolSpec)
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}

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

// Install downloads the official mise installer and runs it so the binary lands at misePath.
// Installs to a sibling temp path then renames into place (atomic replace).
func Install(ctx context.Context, misePath string) error {
	if err := os.MkdirAll(filepath.Dir(misePath), 0755); err != nil {
		return fmt.Errorf("create mise directory: %w", err)
	}

	logger := logging.GetLogger(ctx)
	logger.Info("installing mise", "path", misePath, "url", installerURL)

	httpDriver, err := driver.Get[httpclient.Driver](ctx)
	if err != nil {
		return fmt.Errorf("get http client: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, installerURL, nil)
	if err != nil {
		return fmt.Errorf("create installer request: %w", err)
	}
	resp, err := httpDriver.Client().Do(req)
	if err != nil {
		return fmt.Errorf("download installer: %w", err)
	}
	defer logging.Close(ctx, resp.Body, "url", installerURL)

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("%w: HTTP %d", ErrHTTPDownloadFailed, resp.StatusCode)
	}

	scriptBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("read installer script: %w", err)
	}

	tmpPath := misePath + ".tmp"
	_ = os.Remove(tmpPath)
	defer func() { _ = os.Remove(tmpPath) }()

	installCmd, err := execdriver.Run(ctx, bash.GetShell(ctx), "-s")
	if err != nil {
		return fmt.Errorf("create install command: %w", err)
	}

	installCmd.Stdin = io.NopCloser(bytes.NewReader(scriptBytes))
	installCmd.Stdout = os.Stderr
	installCmd.Stderr = os.Stderr
	installCmd.Env = append(os.Environ(), fmt.Sprintf("MISE_INSTALL_PATH=%s", tmpPath))

	if err := installCmd.Run(); err != nil {
		return fmt.Errorf("run mise installer: %w", err)
	}

	if _, err := os.Stat(tmpPath); err != nil {
		return fmt.Errorf("%w: binary not found at %s", ErrMiseInstallFailed, tmpPath)
	}

	// Atomic replace of the live binary.
	if err := os.Rename(tmpPath, misePath); err != nil {
		_ = os.Remove(misePath)
		if err := os.Rename(tmpPath, misePath); err != nil {
			return fmt.Errorf("install swap %s: %w", misePath, err)
		}
	}

	logger.Info("mise installed successfully", "path", misePath)
	return nil
}
