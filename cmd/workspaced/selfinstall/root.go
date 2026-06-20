package selfinstall

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"workspaced/pkg/driver/shim"
	"workspaced/pkg/logging"
	"workspaced/pkg/taskgroup"
	"workspaced/pkg/version"

	"github.com/spf13/cobra"
)

func GetCommand() *cobra.Command {
	var force bool

	cmd := &cobra.Command{
		Use:   "self-install",
		Short: "Install workspaced into tool system (bootstrap)",
		Long: `Copies the current workspaced binary into the tool management system.

This is typically used once during initial setup:
  curl ... > workspaced && chmod +x workspaced
  ./workspaced self-install

After this, use 'workspaced self-update' to update.

The binary is installed in:
  ~/.local/share/workspaced/tools/github-lucasew-workspaced/{version}/workspaced

A shim is created in:
  ~/.local/bin/workspaced`,
		RunE: func(c *cobra.Command, args []string) error {
			ctx := c.Context()
			g := taskgroup.FromContext(ctx)

			g.Go("self-install", taskgroup.Control, func(ctx context.Context, s *taskgroup.Status) error {
				s.Update("self-installing workspaced")
				s.Progress(0, 1)
				defer s.Progress(1, 1)
				return runSelfInstall(ctx, force)
			})
			return taskgroup.Run(g)
		},
	}

	cmd.Flags().BoolVarP(&force, "force", "f", false, "Force reinstall (overwrite existing)")
	return cmd
}

func runSelfInstall(ctx context.Context, force bool) error {
	currentBinary, err := os.Executable()
	if err != nil {
		return fmt.Errorf("failed to get current binary: %w", err)
	}

	// Determine install location (fixed path, not versioned)
	home, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("failed to get home directory: %w", err)
	}

	installDir := filepath.Join(home, ".local", "share", "workspaced", "bin")
	installPath := filepath.Join(installDir, "workspaced")

	currentVersion := version.Version()

	alreadyInstalled := false
	if !force {
		if _, err := os.Stat(installPath); err == nil {
			alreadyInstalled = true
			logger := logging.GetLogger(ctx)
			logger.Info("already installed", "path", installPath)
		}
	}

	// Copy binary (unless already installed and not forcing)
	if !alreadyInstalled {
		logger := logging.GetLogger(ctx)
		logger.Info("installing workspaced", "version", currentVersion, "path", installPath, "force", force)

		if err := os.MkdirAll(installDir, 0755); err != nil {
			return fmt.Errorf("failed to create install directory: %w", err)
		}

		if err := copyFile(ctx, currentBinary, installPath); err != nil {
			return fmt.Errorf("failed to copy binary: %w", err)
		}

		if err := os.Chmod(installPath, 0755); err != nil {
			return fmt.Errorf("failed to set permissions: %w", err)
		}

		logger.Info("binary installed", "path", installPath)
	}

	// Always regenerate shims (even if binary already installed)
	logger := logging.GetLogger(ctx)
	logger.Info("regenerating shims")

	if err := createWorkspacedShim(ctx, installPath); err != nil {
		return fmt.Errorf("failed to create shim: %w", err)
	}

	if err := createMiseShim(ctx); err != nil {
		logger.Warn("failed to create mise shim", "error", err)
	}

	logger.Info("workspaced installed successfully", "version", currentVersion)
	if alreadyInstalled {
		logger.Info("shims regenerated (use --force to reinstall binary)")
	}
	logger.Info("add ~/.local/bin to your PATH if not already added")

	return nil
}

func createWorkspacedShim(ctx context.Context, workspacedPath string) error {
	home, err := os.UserHomeDir()
	if err != nil {
		return err
	}

	localBin := filepath.Join(home, ".local", "bin")
	if err := os.MkdirAll(localBin, 0755); err != nil {
		return err
	}

	shimPath := filepath.Join(localBin, "workspaced")

	if err := shim.Generate(ctx, shimPath, []string{workspacedPath}); err != nil {
		return err
	}

	logger := logging.GetLogger(ctx)
	logger.Info("created shim", "path", shimPath, "target", workspacedPath)
	return nil
}

func createMiseShim(ctx context.Context) error {
	home, err := os.UserHomeDir()
	if err != nil {
		return err
	}

	// Always create shim, even if mise not installed yet
	misePath := filepath.Join(home, ".local", "share", "workspaced", "bin", "mise")

	localBin := filepath.Join(home, ".local", "bin")
	if err := os.MkdirAll(localBin, 0755); err != nil {
		return err
	}

	shimPath := filepath.Join(localBin, "mise")

	if err := shim.Generate(ctx, shimPath, []string{misePath}); err != nil {
		return err
	}

	logger := logging.GetLogger(ctx)
	logger.Info("created mise shim", "path", shimPath, "target", misePath)
	return nil
}

func copyFile(ctx context.Context, src, dst string) error {
	source, err := os.Open(src)
	if err != nil {
		return err
	}
	defer logging.Close(ctx, source, "path", src)

	dir := filepath.Dir(dst)
	tmp, err := os.CreateTemp(dir, filepath.Base(dst)+".tmp-*")
	if err != nil {
		return err
	}
	tmpPath := tmp.Name()
	defer func() {
		if tmp != nil {
			logging.Close(ctx, tmp, "path", tmpPath)
		}
		logging.RunCleanup(ctx, "remove", func() error {
			if err := os.Remove(tmpPath); err != nil && !os.IsNotExist(err) {
				return err
			}
			return nil
		}, "path", tmpPath)
	}()

	if _, err := io.Copy(tmp, source); err != nil {
		return err
	}

	if err := tmp.Sync(); err != nil {
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	tmp = nil
	if err := os.Chmod(tmpPath, 0755); err != nil {
		return err
	}
	return os.Rename(tmpPath, dst)
}
