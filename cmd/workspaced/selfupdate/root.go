package selfupdate

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"runtime/debug"
	execdriver "workspaced/pkg/driver/exec"
	"workspaced/pkg/env"

	"github.com/spf13/cobra"
)

func NewCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "self-update",
		Short: "Update workspaced binary",
		Long: `Update workspaced binary to the latest version.

This command automatically detects the update strategy:
  1. If source exists → rebuild using mise
     Checked locations:
     - ~/.config/workspaced/src/
     - <dotfiles_root>/workspaced/ (from GetDotfilesRoot)
  2. Otherwise → download latest release from GitHub

The binary is updated at ~/.local/share/workspaced/bin/workspaced`,
		RunE: func(c *cobra.Command, args []string) error {
			return runSelfUpdate(c)
		},
	}

	return cmd
}

func runSelfUpdate(cmd *cobra.Command) error {
	// Get install path from env driver
	dataDir, err := env.GetUserDataDir()
	if err != nil {
		return fmt.Errorf("failed to get data directory: %w", err)
	}
	installPath := filepath.Join(dataDir, "bin", "workspaced")

	// Try to find source code in common locations
	var sourcePaths []string

	// 1. ~/.config/workspaced/src/
	if configDir, err := env.GetConfigDir(); err == nil {
		sourcePaths = append(sourcePaths, filepath.Join(configDir, "src"))
	}

	// 2. $DOTFILES/workspaced/
	if dotfilesRoot, err := env.GetDotfilesRoot(); err == nil {
		sourcePaths = append(sourcePaths, filepath.Join(dotfilesRoot, "workspaced"))
	}

	// Check each location and build from first that exists
	for _, srcPath := range sourcePaths {
		if _, err := os.Stat(srcPath); err == nil {
			fmt.Printf("📦 Building from source at %s...\n", srcPath)
			return buildFromSource(cmd, srcPath, installPath)
		}
	}

	// No source found, download from GitHub
	fmt.Printf("📦 Downloading latest release from GitHub...\n")
	return downloadFromGitHub(installPath)
}

func buildFromSource(cmd *cobra.Command, srcDir, installPath string) error {
	ctx := cmd.Context()

	// Get the Go version used to build the current binary
	goVersion := getGoVersion()
	if goVersion == "" {
		return fmt.Errorf("could not determine Go version from build info")
	}

	// Get mise path using the same approach as workspaced open mise
	misePath, err := getMisePath()
	if err != nil {
		return fmt.Errorf("mise required to build from source: %w", err)
	}

	// Create install directory
	if err := os.MkdirAll(filepath.Dir(installPath), 0755); err != nil {
		return fmt.Errorf("failed to create install directory: %w", err)
	}

	// Build directly to install path (go build is already atomic)
	goSpec := fmt.Sprintf("go@%s", goVersion)

	// Use execdriver like workspaced open mise does
	buildCmd, err := execdriver.Run(ctx, misePath, "exec", goSpec, "--", "go", "build", "-o", installPath, "./cmd/workspaced")
	if err != nil {
		return fmt.Errorf("failed to create build command: %w", err)
	}

	buildCmd.Dir = srcDir
	buildCmd.Stdout = os.Stdout
	buildCmd.Stderr = os.Stderr

	fmt.Printf("   Running: mise exec %s -- go build -o %s ./cmd/workspaced\n", goSpec, installPath)
	if err := buildCmd.Run(); err != nil {
		return fmt.Errorf("build failed: %w", err)
	}

	fmt.Printf("   ✓ Built and installed successfully\n")
	return nil
}

func getGoVersion() string {
	info, ok := debug.ReadBuildInfo()
	if !ok {
		return ""
	}

	// GoVersion is like "go1.26.0", we need just "1.26.0"
	version := info.GoVersion
	if len(version) > 2 && version[0] == 'g' && version[1] == 'o' {
		return version[2:]
	}
	return version
}

func getMisePath() (string, error) {
	if path := os.Getenv("MISE_INSTALL_PATH"); path != "" {
		return path, nil
	}

	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}

	misePath := filepath.Join(home, ".local", "share", "workspaced", "bin", "mise")
	if _, err := os.Stat(misePath); err != nil {
		return "", fmt.Errorf("mise not found at %s: %w", misePath, err)
	}

	return misePath, nil
}

func downloadFromGitHub(installPath string) error {
	// Determine platform and architecture
	goos := runtime.GOOS
	goarch := runtime.GOARCH

	// Map Go arch to common naming
	arch := goarch
	if goarch == "amd64" {
		arch = "x86_64"
	} else if goarch == "arm64" {
		arch = "aarch64"
	}

	// Construct release URL (assuming GitHub releases follow a pattern)
	// Format: workspaced-{os}-{arch}
	releaseFileName := fmt.Sprintf("workspaced-%s-%s", goos, arch)
	releaseURL := fmt.Sprintf("https://github.com/lucasew/workspaced/releases/latest/download/%s", releaseFileName)

	fmt.Printf("   Downloading from: %s\n", releaseURL)

	resp, err := http.Get(releaseURL)
	if err != nil {
		return fmt.Errorf("failed to download release: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("failed to download release: HTTP %d\n\nNote: GitHub releases may not be available yet.\nPlease use source build method instead.", resp.StatusCode)
	}

	// Create install directory
	if err := os.MkdirAll(filepath.Dir(installPath), 0755); err != nil {
		return fmt.Errorf("failed to create install directory: %w", err)
	}

	// Download to temporary file first
	tmpFile := installPath + ".tmp"
	out, err := os.Create(tmpFile)
	if err != nil {
		return fmt.Errorf("failed to create temporary file: %w", err)
	}
	defer out.Close()

	if _, err := io.Copy(out, resp.Body); err != nil {
		os.Remove(tmpFile)
		return fmt.Errorf("failed to write binary: %w", err)
	}

	// Close file before rename
	out.Close()

	// Set executable permissions
	if err := os.Chmod(tmpFile, 0755); err != nil {
		os.Remove(tmpFile)
		return fmt.Errorf("failed to set permissions: %w", err)
	}

	// Rename to final location
	if err := os.Rename(tmpFile, installPath); err != nil {
		os.Remove(tmpFile)
		return fmt.Errorf("failed to install binary: %w", err)
	}

	fmt.Printf("   ✓ Downloaded and installed successfully\n")
	return nil
}
