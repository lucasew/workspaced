package selfinstall

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
)

func NewCommand() *cobra.Command {
	var force bool

	cmd := &cobra.Command{
		Use:   "self-install",
		Short: "Install workspaced binary to the system",
		Long: `Install workspaced binary and setup PATH.

This command will:
  1. Copy the binary to ~/.local/share/workspaced/bin/workspaced
  2. Add workspaced to your PATH (modifies shell config)

After installation, restart your shell or run:
  source ~/.bashrc  # or ~/.zshrc

To initialize your dotfiles after installation:
  workspaced init`,
		RunE: func(c *cobra.Command, args []string) error {
			return runSelfInstall(force)
		},
	}

	cmd.Flags().BoolVarP(&force, "force", "f", false, "Force reinstall (overwrite existing)")

	return cmd
}

func runSelfInstall(force bool) error {
	home, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("failed to get home directory: %w", err)
	}

	// 1. Install binary
	fmt.Printf("📦 Installing workspaced binary...\n")
	installDir := filepath.Join(home, ".local", "share", "workspaced", "bin")
	installPath := filepath.Join(installDir, "workspaced")

	if !force {
		if _, err := os.Stat(installPath); err == nil {
			fmt.Printf("   ✓ Already installed at %s\n", installPath)
		} else {
			if err := installBinary(installPath); err != nil {
				return err
			}
		}
	} else {
		if err := installBinary(installPath); err != nil {
			return err
		}
	}

	// 2. Setup PATH
	fmt.Printf("\n🔧 Setting up PATH...\n")
	if err := setupPath(home, installDir); err != nil {
		fmt.Printf("   ⚠️  Warning: %v\n", err)
		fmt.Printf("   Please manually add to your PATH:\n")
		fmt.Printf("   export PATH=\"%s:$PATH\"\n", installDir)
	}

	// 3. Success message
	fmt.Printf("\n✅ Binary installation complete!\n\n")
	fmt.Printf("Next steps:\n")
	fmt.Printf("  1. Restart your shell or run: source ~/.bashrc\n")
	fmt.Printf("  2. Verify installation: workspaced --version\n")
	fmt.Printf("  3. Initialize dotfiles: workspaced init\n")

	return nil
}

func installBinary(installPath string) error {
	installDir := filepath.Dir(installPath)
	if err := os.MkdirAll(installDir, 0755); err != nil {
		return fmt.Errorf("failed to create installation directory: %w", err)
	}

	currentBinary, err := os.Executable()
	if err != nil {
		return fmt.Errorf("failed to get current executable path: %w", err)
	}

	fmt.Printf("   From: %s\n", currentBinary)
	fmt.Printf("   To:   %s\n", installPath)

	if err := copyFile(currentBinary, installPath); err != nil {
		return fmt.Errorf("failed to copy binary: %w", err)
	}

	if err := os.Chmod(installPath, 0755); err != nil {
		return fmt.Errorf("failed to set executable permissions: %w", err)
	}

	fmt.Printf("   ✓ Binary installed\n")
	return nil
}

func copyFile(src, dst string) error {
	source, err := os.Open(src)
	if err != nil {
		return err
	}
	defer source.Close()

	destination, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer destination.Close()

	_, err = io.Copy(destination, source)
	return err
}

func setupPath(home, installDir string) error {
	pathExport := fmt.Sprintf("\n# Added by workspaced self-install\nexport PATH=\"%s:$PATH\"\n", installDir)

	shells := []string{
		filepath.Join(home, ".bashrc"),
		filepath.Join(home, ".zshrc"),
		filepath.Join(home, ".profile"),
	}

	modified := false
	for _, shellConfig := range shells {
		if _, err := os.Stat(shellConfig); err == nil {
			content, err := os.ReadFile(shellConfig)
			if err != nil {
				continue
			}

			if !strings.Contains(string(content), installDir) {
				f, err := os.OpenFile(shellConfig, os.O_APPEND|os.O_WRONLY, 0644)
				if err != nil {
					continue
				}
				defer f.Close()

				if _, err := f.WriteString(pathExport); err != nil {
					continue
				}

				fmt.Printf("   ✓ Added to %s\n", shellConfig)
				modified = true
			} else {
				fmt.Printf("   ✓ Already in %s\n", shellConfig)
				modified = true
			}
		}
	}

	if !modified {
		return fmt.Errorf("no shell config found")
	}

	return nil
}
