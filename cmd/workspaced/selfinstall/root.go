package selfinstall

import (
	"context"
	"io"
	"os"
	"path/filepath"

	"github.com/lucasew/workspaced/internal/atomicfile"
	"github.com/lucasew/workspaced/internal/version"
	envdriver "github.com/lucasew/workspaced/pkg/driver/env"
	"github.com/lucasew/workspaced/pkg/driver/shim"
	"github.com/lucasew/workspaced/pkg/logging"
	"github.com/lucasew/workspaced/pkg/taskgroup"

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
				defer s.Unit()()
				return runSelfInstall(ctx, force)
			})
			return nil
		},
	}

	cmd.Flags().BoolVarP(&force, "force", "f", false, "Force reinstall (overwrite existing)")
	return cmd
}

func runSelfInstall(ctx context.Context, force bool) error {
	currentBinary, err := os.Executable()
	if err != nil {
		return fmt.Errorf("get current binary: %w", err)
	}

	// Fixed install location under real home (not Termux proot /home view).
	dataDir, err := envdriver.GetUserDataDir(ctx)
	if err != nil {
		// Bootstrap before drivers/weights: fall back to ResolveHomeDir.
		home, homeErr := envdriver.ResolveHomeDir()
		if homeErr != nil {
			return fmt.Errorf("get home directory: %w", err)
		}
		dataDir = filepath.Join(home, ".local", "share", "workspaced")
		if mkErr := os.MkdirAll(dataDir, 0o755); mkErr != nil {
			return fmt.Errorf("create user data dir: %w", mkErr)
		}
	}

	installDir := filepath.Join(dataDir, "bin")
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
			return fmt.Errorf("create install directory: %w", err)
		}

		if err := copyFile(ctx, currentBinary, installPath); err != nil {
			return fmt.Errorf("copy binary: %w", err)
		}

		if err := os.Chmod(installPath, 0755); err != nil {
			return fmt.Errorf("set permissions: %w", err)
		}

		logger.Info("binary installed", "path", installPath)
	}

	// Always regenerate shims (even if binary already installed)
	logger := logging.GetLogger(ctx)
	logger.Info("regenerating shims")

	if err := createWorkspacedShim(ctx, installPath); err != nil {
		return fmt.Errorf("create shim: %w", err)
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
	shimPath, err := shim.GenerateInLocalBin(ctx, "workspaced", []string{workspacedPath})
	if err != nil {
		return err
	}
	logging.GetLogger(ctx).Info("created shim", "path", shimPath, "target", workspacedPath)
	return nil
}

func createMiseShim(ctx context.Context) error {
	// Always create shim, even if mise not installed yet.
	dataDir, err := envdriver.GetUserDataDir(ctx)
	if err != nil {
		home, homeErr := envdriver.ResolveHomeDir()
		if homeErr != nil {
			return err
		}
		dataDir = filepath.Join(home, ".local", "share", "workspaced")
	}
	misePath := filepath.Join(dataDir, "bin", "mise")
	shimPath, err := shim.GenerateInLocalBin(ctx, "mise", []string{misePath})
	if err != nil {
		return err
	}
	logging.GetLogger(ctx).Info("created mise shim", "path", shimPath, "target", misePath)
	return nil
}

func copyFile(ctx context.Context, src, dst string) error {
	source, err := os.Open(src)
	if err != nil {
		return err
	}
	defer logging.Close(ctx, source, "path", src)

	f, err := atomicfile.Create(dst, 0o755)
	if err != nil {
		return err
	}
	defer f.Abort()
	if _, err := io.Copy(f, source); err != nil {
		return err
	}
	return f.CommitMode(0o755)
}
