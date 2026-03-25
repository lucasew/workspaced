package miseutil

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"workspaced/pkg/driver"
	execdriver "workspaced/pkg/driver/exec"
	"workspaced/pkg/driver/httpclient"
)

func GetPath() string {
	if path := os.Getenv("MISE_INSTALL_PATH"); path != "" {
		return path
	}

	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}

	return filepath.Join(home, ".local", "share", "workspaced", "bin", "mise")
}

func Ensure(ctx context.Context) (string, error) {
	misePath := GetPath()
	if misePath == "" {
		return "", fmt.Errorf("could not determine mise install path")
	}

	if _, err := os.Stat(misePath); err == nil {
		return misePath, nil
	}

	if err := install(ctx, misePath); err != nil {
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

	candidates := []string{
		filepath.Join(root, "bin", binName),
		filepath.Join(root, "bin", binName+".exe"),
		filepath.Join(root, binName),
		filepath.Join(root, binName+".exe"),
	}

	for _, candidate := range candidates {
		if _, err := os.Stat(candidate); err == nil {
			return candidate, nil
		}
	}

	return "", fmt.Errorf("binary %q not found under %s", binName, root)
}

func install(ctx context.Context, misePath string) error {
	if err := os.MkdirAll(filepath.Dir(misePath), 0755); err != nil {
		return fmt.Errorf("failed to create mise directory: %w", err)
	}

	httpDriver, err := driver.Get[httpclient.Driver](ctx)
	if err != nil {
		return fmt.Errorf("failed to get http client: %w", err)
	}

	resp, err := httpDriver.Client().Get("https://mise.run")
	if err != nil {
		return fmt.Errorf("failed to download installer: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("failed to download installer: HTTP %d", resp.StatusCode)
	}

	scriptBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read installer script: %w", err)
	}

	installCmd, err := execdriver.Run(ctx, "bash", "-s")
	if err != nil {
		return fmt.Errorf("failed to create install command: %w", err)
	}

	installCmd.Stdin = io.NopCloser(bytes.NewReader(scriptBytes))
	installCmd.Stdout = os.Stderr
	installCmd.Stderr = os.Stderr
	installCmd.Env = append(os.Environ(), fmt.Sprintf("MISE_INSTALL_PATH=%s", misePath))

	if err := installCmd.Run(); err != nil {
		return fmt.Errorf("failed to install mise: %w", err)
	}

	if _, err := os.Stat(misePath); err != nil {
		return fmt.Errorf("mise installation failed - binary not found at %s", misePath)
	}

	return nil
}
