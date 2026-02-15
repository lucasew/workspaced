package selfinstall

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"workspaced/pkg/driver/shim"
	"workspaced/pkg/tool"
	"workspaced/pkg/tool/resolution"
	"workspaced/pkg/version"

	"github.com/spf13/cobra"
)

func NewCommand() *cobra.Command {
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
			return runSelfInstall(c.Context())
		},
	}

	return cmd
}

func runSelfInstall(ctx context.Context) error {
	// Get current binary path
	currentBinary, err := os.Executable()
	if err != nil {
		return fmt.Errorf("failed to get current binary: %w", err)
	}

	// Determine install location
	toolsDir, err := tool.GetToolsDir()
	if err != nil {
		return err
	}

	currentVersion := version.Version()
	toolDir := filepath.Join(toolsDir, "github-lucasew-workspaced", currentVersion)
	installPath := filepath.Join(toolDir, "workspaced")

	// Copy binary
	slog.Info("installing workspaced", "version", currentVersion, "path", toolDir)

	if err := os.MkdirAll(toolDir, 0755); err != nil {
		return err
	}

	if err := copyFile(currentBinary, installPath); err != nil {
		return fmt.Errorf("failed to copy binary: %w", err)
	}

	if err := os.Chmod(installPath, 0755); err != nil {
		return err
	}

	slog.Info("binary installed", "path", installPath)

	// Create workspaced shim
	if err := createWorkspacedShim(ctx, toolsDir); err != nil {
		return fmt.Errorf("failed to create shim: %w", err)
	}

	// Create mise shim (if mise exists)
	if err := createMiseShim(ctx); err != nil {
		slog.Warn("failed to create mise shim", "error", err)
	}

	slog.Info("workspaced installed successfully", "version", currentVersion)
	slog.Info("add ~/.local/bin to your PATH if not already added")

	return nil
}

func createWorkspacedShim(ctx context.Context, toolsDir string) error {
	home, err := os.UserHomeDir()
	if err != nil {
		return err
	}

	localBin := filepath.Join(home, ".local", "bin")
	if err := os.MkdirAll(localBin, 0755); err != nil {
		return err
	}

	shimPath := filepath.Join(localBin, "workspaced")

	// Resolve workspaced via tool resolution
	resolver := resolution.NewResolver(toolsDir)
	workspacedPath, err := resolver.Resolve(ctx, "workspaced")
	if err != nil {
		return fmt.Errorf("failed to resolve workspaced: %w", err)
	}

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

	// Check if mise exists
	dataDir, err := tool.GetToolsDir()
	if err != nil {
		return err
	}

	misePath := filepath.Join(filepath.Dir(dataDir), "bin", "mise")
	if _, err := os.Stat(misePath); err != nil {
		// Mise not installed, skip
		return nil
	}

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
