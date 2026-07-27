package selfbin

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"runtime"

	envdriver "github.com/lucasew/workspaced/pkg/driver/env"
	"github.com/lucasew/workspaced/pkg/driver/shim"
	"github.com/lucasew/workspaced/pkg/logging"
)

// InstallPaths returns the fixed bin dir and workspaced binary path under the
// env driver's user data dir (ResolveHomeDir fallback during bootstrap).
func InstallPaths(ctx context.Context) (installDir, installPath string, err error) {
	dataDir, err := envdriver.GetUserDataDir(ctx)
	if err != nil {
		home, homeErr := envdriver.ResolveHomeDir()
		if homeErr != nil {
			return "", "", fmt.Errorf("get home directory: %w", err)
		}
		dataDir = filepath.Join(home, ".local", "share", "workspaced")
		if mkErr := os.MkdirAll(dataDir, 0o755); mkErr != nil {
			return "", "", fmt.Errorf("create user data dir: %w", mkErr)
		}
	}
	installDir = filepath.Join(dataDir, "bin")
	name := "workspaced"
	if runtime.GOOS == "windows" {
		name = "workspaced.exe"
	}
	installPath = filepath.Join(installDir, name)
	return installDir, installPath, nil
}

// EnsureWorkspacedShim writes ~/.local/bin/workspaced → workspacedPath.
func EnsureWorkspacedShim(ctx context.Context, workspacedPath string) error {
	shimPath, err := shim.GenerateInLocalBin(ctx, "workspaced", []string{workspacedPath})
	if err != nil {
		return err
	}
	logging.GetLogger(ctx).Info("workspaced shim ready", "path", shimPath, "target", workspacedPath)
	return nil
}
