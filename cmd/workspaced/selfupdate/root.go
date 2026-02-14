package selfupdate

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"runtime/debug"
	"strings"
	"workspaced/pkg/driver"
	execdriver "workspaced/pkg/driver/exec"
	"workspaced/pkg/driver/fetchurl"
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
	_ = ctx // Keep context for potential future use

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

type githubAsset struct {
	Name               string `json:"name"`
	Digest             string `json:"digest"`
	BrowserDownloadURL string `json:"browser_download_url"`
}

type githubRelease struct {
	TagName string        `json:"tag_name"`
	Assets  []githubAsset `json:"assets"`
}

func downloadFromGitHub(installPath string) error {
	ctx := context.Background()

	// Determine platform and architecture
	goos := runtime.GOOS
	goarch := runtime.GOARCH
	releaseFileName := fmt.Sprintf("workspaced-%s-%s", goos, goarch)

	// Fetch release info from GitHub API
	apiURL := "https://api.github.com/repos/lucasew/workspaced/releases/latest"
	fmt.Printf("   Fetching release info from GitHub API...\n")

	resp, err := http.Get(apiURL)
	if err != nil {
		return fmt.Errorf("failed to fetch release info: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("failed to fetch release info: HTTP %d\n\nNote: GitHub releases may not be available yet.\nPlease use source build method instead.", resp.StatusCode)
	}

	var release githubRelease
	if err := json.NewDecoder(resp.Body).Decode(&release); err != nil {
		return fmt.Errorf("failed to parse release info: %w", err)
	}

	// Find the matching asset
	var asset *githubAsset
	for i := range release.Assets {
		if release.Assets[i].Name == releaseFileName {
			asset = &release.Assets[i]
			break
		}
	}

	if asset == nil {
		return fmt.Errorf("asset not found: %s\n\nAvailable assets: %v", releaseFileName, getAssetNames(release.Assets))
	}

	// Extract hash from digest (format: "sha256:HASH")
	var hash, algo string
	if asset.Digest != "" {
		parts := strings.SplitN(asset.Digest, ":", 2)
		if len(parts) == 2 {
			algo = parts[0]
			hash = parts[1]
			fmt.Printf("   Found %s checksum: %s\n", algo, hash[:16]+"...")
		}
	}

	fmt.Printf("   Downloading %s (%s)...\n", asset.Name, release.TagName)

	// Create install directory
	if err := os.MkdirAll(filepath.Dir(installPath), 0755); err != nil {
		return fmt.Errorf("failed to create install directory: %w", err)
	}

	// Download to temporary file with hash verification
	tmpFile := installPath + ".tmp"
	out, err := os.Create(tmpFile)
	if err != nil {
		return fmt.Errorf("failed to create temporary file: %w", err)
	}
	defer out.Close()

	// Use fetchurl driver for verified download
	fetcher, err := driver.Get[fetchurl.Driver](ctx)
	if err != nil {
		return fmt.Errorf("failed to get fetchurl driver: %w", err)
	}

	if err := fetcher.Fetch(ctx, fetchurl.FetchOptions{
		URLs: []string{asset.BrowserDownloadURL},
		Algo: algo,
		Hash: hash,
		Out:  out,
	}); err != nil {
		os.Remove(tmpFile)
		return fmt.Errorf("failed to download binary: %w", err)
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

	fmt.Printf("   ✓ Downloaded and installed successfully (hash verified)\n")
	return nil
}

func getAssetNames(assets []githubAsset) []string {
	names := make([]string, len(assets))
	for i, a := range assets {
		names[i] = a.Name
	}
	return names
}
