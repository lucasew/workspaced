package selfinstall

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"workspaced/pkg/driver/shim"
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
			return runSelfInstall(c.Context(), force)
		},
	}

	cmd.Flags().BoolVarP(&force, "force", "f", false, "Force reinstall (overwrite existing)")
	return cmd
}

func runSelfInstall(ctx context.Context, force bool) error {
	// Get current binary path
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

	// Check if already installed
	alreadyInstalled := false
	if !force {
		if _, err := os.Stat(installPath); err == nil {
			alreadyInstalled = true
			slog.Info("already installed", "path", installPath)
		}
	}

	// Copy binary (unless already installed and not forcing)
	if !alreadyInstalled {
		slog.Info("installing workspaced", "version", currentVersion, "path", installPath, "force", force)

		if err := os.MkdirAll(installDir, 0755); err != nil {
			return fmt.Errorf("failed to create install directory: %w", err)
		}

		if err := copyFile(currentBinary, installPath); err != nil {
			return fmt.Errorf("failed to copy binary: %w", err)
		}

		if err := os.Chmod(installPath, 0755); err != nil {
			return fmt.Errorf("failed to set permissions: %w", err)
		}

		slog.Info("binary installed", "path", installPath)
	}

	// Always regenerate shims (even if binary already installed)
	slog.Info("regenerating shims")

	if err := createWorkspacedShim(ctx, installPath); err != nil {
		return fmt.Errorf("failed to create shim: %w", err)
	}

	if err := createMiseShim(ctx); err != nil {
		slog.Warn("failed to create mise shim", "error", err)
	}

	slog.Info("workspaced installed successfully", "version", currentVersion)
	if alreadyInstalled {
		slog.Info("shims regenerated (use --force to reinstall binary)")
	}
	slog.Info("add ~/.local/bin to your PATH if not already added")

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

	// Use shimdriver
	if err := shim.Generate(ctx, shimPath, []string{workspacedPath}); err != nil {
		return err
	}

	slog.Info("created shim", "path", shimPath, "target", workspacedPath)
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

	// Create shim pointing to mise binary
	if err := shim.Generate(ctx, shimPath, []string{misePath}); err != nil {
		return err
	}

	slog.Info("created mise shim", "path", shimPath, "target", misePath)
	return nil
}

func copyFile(src, dst string) error {
	source, err := os.Open(src)
	if err != nil {
		return err
	}
	defer source.Close()

	dest, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer dest.Close()

	if _, err := io.Copy(dest, source); err != nil {
		return err
	}

	return dest.Sync()
}
