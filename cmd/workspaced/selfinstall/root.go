package selfinstall

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/lucasew/workspaced/internal/atomicfile"
	"github.com/lucasew/workspaced/internal/miseutil"
	"github.com/lucasew/workspaced/internal/selfbin"
	"github.com/lucasew/workspaced/internal/version"
	envdriver "github.com/lucasew/workspaced/pkg/driver/env"
	"github.com/lucasew/workspaced/pkg/logging"
	"github.com/lucasew/workspaced/pkg/taskgroup"

	"github.com/lewtec/lewkit/x/cmd"
)

type Command struct {
	Force cmd.Flag `short:"f" long:"force" help:"Force reinstall (overwrite existing)"`
}

func (Command) Description() string {
	return "Install workspaced into tool system (bootstrap)"
}

func (c *Command) Run(ctx context.Context) error {
	g := taskgroup.FromContext(ctx)

	g.Go("self-install", taskgroup.Control, func(ctx context.Context, s *taskgroup.Status) error {
		s.Update("self-installing workspaced")
		defer s.Unit()()
		return runSelfInstall(ctx, c.Force.Value())
	})
	return nil
}

func runSelfInstall(ctx context.Context, force bool) error {
	currentBinary, err := os.Executable()
	if err != nil {
		return fmt.Errorf("get current binary: %w", err)
	}

	// Fixed install location under real home (not Termux proot /home view).
	installDir, installPath, err := selfbin.InstallPaths(ctx)
	if err != nil {
		return err
	}

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

	if err := selfbin.EnsureWorkspacedShim(ctx, installPath); err != nil {
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

func createMiseShim(ctx context.Context) error {
	// Integration shim only: re-enters the standard lazy route for mise.
	// Does not install mise; open lazy --home resolves lazy_tools.mise.
	dataDir, err := envdriver.GetUserDataDir(ctx)
	if err != nil {
		home, homeErr := envdriver.ResolveHomeDir()
		if homeErr != nil {
			return err
		}
		dataDir = filepath.Join(home, ".local", "share", "workspaced")
	}
	workspacedBin := filepath.Join(dataDir, "bin", "workspaced")
	if err := miseutil.EnsureLocalBinWrapper(ctx, workspacedBin); err != nil {
		return err
	}
	logging.GetLogger(ctx).Info("created mise wrapper", "target", "open lazy --home mise")
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
	defer logging.RunCleanup(ctx, "atomicfile.Abort", f.Abort)
	if _, err := io.Copy(f, source); err != nil {
		return err
	}
	return f.CommitMode(0o755)
}
