package open

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	execdriver "workspaced/pkg/driver/exec"

	"github.com/spf13/cobra"
)

// getMisePath returns the path where mise should be installed.
// Priority: MISE_INSTALL_PATH env var > ~/.local/share/workspaced/bin/mise
func getMisePath() string {
	if path := os.Getenv("MISE_INSTALL_PATH"); path != "" {
		return path
	}

	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}

	return filepath.Join(home, ".local", "share", "workspaced", "bin", "mise")
}

// installMise downloads and installs mise using the official installer.
func installMise(ctx *cobra.Command) error {
	misePath := getMisePath()
	if misePath == "" {
		return fmt.Errorf("could not determine mise install path")
	}

	// Create directory if it doesn't exist
	miseDir := filepath.Dir(misePath)
	if err := os.MkdirAll(miseDir, 0755); err != nil {
		return fmt.Errorf("failed to create mise directory: %w", err)
	}

	fmt.Fprintf(os.Stderr, "Installing mise to %s...\n", misePath)

	// Download installer script using Go's HTTP client (no curl/wget needed!)
	fmt.Fprintf(os.Stderr, "Downloading installer from https://mise.run...\n")
	resp, err := http.Get("https://mise.run")
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

	// Run the installer script via sh
	installCmd, err := execdriver.Run(ctx.Context(), "sh", "-s")
	if err != nil {
		return fmt.Errorf("failed to create install command: %w", err)
	}

	// Pipe the script to sh's stdin
	installCmd.Stdin = io.NopCloser(bytes.NewReader(scriptBytes))
	installCmd.Stdout = os.Stderr
	installCmd.Stderr = os.Stderr
	installCmd.Env = append(os.Environ(), fmt.Sprintf("MISE_INSTALL_PATH=%s", misePath))

	if err := installCmd.Run(); err != nil {
		return fmt.Errorf("failed to install mise: %w", err)
	}

	// Verify installation
	if _, err := os.Stat(misePath); os.IsNotExist(err) {
		return fmt.Errorf("mise installation failed - binary not found at %s", misePath)
	}

	fmt.Fprintf(os.Stderr, "✓ mise installed successfully\n")
	return nil
}

// ensureMise checks if mise is installed and installs it if needed.
func ensureMise(ctx *cobra.Command) (string, error) {
	misePath := getMisePath()
	if misePath == "" {
		return "", fmt.Errorf("could not determine mise install path")
	}

	// Check if mise exists
	if _, err := os.Stat(misePath); os.IsNotExist(err) {
		if err := installMise(ctx); err != nil {
			return "", err
		}
	}

	return misePath, nil
}

func miseCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "mise [args...]",
		Short: "Run mise (installs automatically if needed)",
		Long: `Run mise using a custom installation path.

This command ensures mise is installed in a location that works on all platforms,
including Termux where ~/.local/bin can cause issues.

Installation path priority:
  1. MISE_INSTALL_PATH environment variable
  2. ~/.local/share/workspaced/bin/mise (default)

If mise is not found, it will be automatically installed using the official
installer from https://mise.run/

Examples:
  workspaced open mise --version
  workspaced open mise install node@20
  workspaced open mise use -g python@3.11
  MISE_INSTALL_PATH=/custom/path/mise workspaced open mise --version`,
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()

			// Ensure mise is installed
			misePath, err := ensureMise(cmd)
			if err != nil {
				return err
			}

			// Create command using driver
			miseCmd, err := execdriver.Run(ctx, misePath, args...)
			if err != nil {
				return fmt.Errorf("failed to create command: %w", err)
			}

			// Connect stdio
			miseCmd.Stdin = os.Stdin
			miseCmd.Stdout = os.Stdout
			miseCmd.Stderr = os.Stderr

			// Run and return exit code
			if err := miseCmd.Run(); err != nil {
				return err
			}

			return nil
		},
	}
}
