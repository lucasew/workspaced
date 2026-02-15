package selfupdate

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"runtime/debug"
	"strings"
	"workspaced/pkg/driver"
	execdriver "workspaced/pkg/driver/exec"
	"workspaced/pkg/driver/httpclient"
	"workspaced/pkg/driver/shim"
	"workspaced/pkg/env"
	"workspaced/pkg/tool"
	"workspaced/pkg/tool/provider"
	"workspaced/pkg/tool/resolution"
	"workspaced/pkg/version"

	"github.com/spf13/cobra"
)

// ============================================================================
// Command
// ============================================================================

func NewCommand() *cobra.Command {
	var force bool

	cmd := &cobra.Command{
		Use:   "self-update",
		Short: "Update workspaced binary",
		Long: `Update workspaced to the latest version.

Strategy:
  1. If source code exists → rebuild from source (always)
  2. Otherwise → download from GitHub using tool provider

The update is installed in ~/.local/share/workspaced/tools/ and the shim
in ~/.local/bin/workspaced is updated automatically.`,
		RunE: func(c *cobra.Command, args []string) error {
			return runSelfUpdate(c.Context(), force)
		},
	}

	cmd.Flags().BoolVar(&force, "force", false, "Force update even if version matches (GitHub only)")
	return cmd
}

// ============================================================================
// Main update flow
// ============================================================================

func runSelfUpdate(ctx context.Context, force bool) error {
	// Try source build first (dev mode - always rebuilds)
	if srcPath, found := findSourcePath(); found {
		slog.Info("building from source (always rebuilds)", "path", srcPath)
		return buildAndInstallFromSource(ctx, srcPath)
	}

	// Fallback to GitHub provider (checks version unless --force)
	return updateFromGitHub(ctx, force)
}

// ============================================================================
// Source build strategy
// ============================================================================

func buildAndInstallFromSource(ctx context.Context, srcPath string) error {
	toolsDir, err := tool.GetToolsDir()
	if err != nil {
		return err
	}

	// Read version from source
	sourceVersion, err := readVersionFile(filepath.Join(srcPath, "pkg/version/version.txt"))
	if err != nil {
		return err
	}

	toolDir := filepath.Join(toolsDir, "github-lucasew-workspaced", sourceVersion)
	installPath := filepath.Join(toolDir, "workspaced")

	// Get build dependencies
	goVersion := getGoVersion()
	if goVersion == "" {
		return fmt.Errorf("could not determine Go version from build info")
	}

	// Ensure mise is installed (auto-install if needed)
	misePath, err := ensureMise(ctx)
	if err != nil {
		return fmt.Errorf("failed to ensure mise: %w", err)
	}

	// Prepare build directory
	if err := os.MkdirAll(toolDir, 0755); err != nil {
		return err
	}

	// Build
	goSpec := fmt.Sprintf("go@%s", goVersion)
	buildCmd, err := execdriver.Run(ctx, misePath, "exec", goSpec, "--",
		"go", "build", "-o", installPath, "./cmd/workspaced")
	if err != nil {
		return err
	}

	buildCmd.Dir = srcPath
	buildCmd.Stdout = os.Stdout
	buildCmd.Stderr = os.Stderr

	slog.Info("building from source", "version", sourceVersion, "go", goSpec)
	if err := buildCmd.Run(); err != nil {
		return fmt.Errorf("build failed: %w", err)
	}

	slog.Info("build completed", "path", installPath)
	return createWorkspacedShim(ctx, toolsDir)
}

// ============================================================================
// GitHub provider strategy
// ============================================================================

func updateFromGitHub(ctx context.Context, force bool) error {
	// Get GitHub provider
	githubProvider, err := tool.GetProvider("github")
	if err != nil {
		return err
	}

	pkg, err := githubProvider.ParsePackage("lucasew/workspaced")
	if err != nil {
		return err
	}

	// Get latest version
	versions, err := githubProvider.ListVersions(ctx, pkg)
	if err != nil {
		return err
	}

	if len(versions) == 0 {
		return fmt.Errorf("no versions found")
	}

	latestVersion := versions[0]
	normalizedLatest := strings.TrimPrefix(latestVersion, "v")

	// Check if update is needed (unless --force)
	if !force {
		currentVersion := version.Version()

		if currentVersion == normalizedLatest {
			slog.Info("already at latest version", "version", currentVersion)
			return nil
		}

		slog.Info("updating", "current", currentVersion, "latest", normalizedLatest)
	} else {
		slog.Info("forcing update", "version", latestVersion)
	}

	// Get artifacts
	artifacts, err := githubProvider.GetArtifacts(ctx, pkg, latestVersion)
	if err != nil {
		return err
	}

	// Find matching artifact for current platform
	artifact := findMatchingArtifact(artifacts, runtime.GOOS, runtime.GOARCH)
	if artifact == nil {
		available := []string{}
		for _, a := range artifacts {
			available = append(available, fmt.Sprintf("%s/%s", a.OS, a.Arch))
		}
		return fmt.Errorf("no artifact found for %s/%s (available: %v)", runtime.GOOS, runtime.GOARCH, available)
	}

	// Install to tools directory
	toolsDir, err := tool.GetToolsDir()
	if err != nil {
		return err
	}

	toolDir := filepath.Join(toolsDir, "github-lucasew-workspaced", normalizedLatest)
	if err := os.MkdirAll(toolDir, 0755); err != nil {
		return err
	}

	slog.Info("downloading from GitHub", "version", latestVersion, "os", artifact.OS, "arch", artifact.Arch)
	if err := githubProvider.Install(ctx, *artifact, toolDir); err != nil {
		return fmt.Errorf("installation failed: %w", err)
	}

	slog.Info("download completed", "path", toolDir)
	return createWorkspacedShim(ctx, toolsDir)
}

func findMatchingArtifact(artifacts []provider.Artifact, os, arch string) *provider.Artifact {
	for i := range artifacts {
		if artifacts[i].OS == os && artifacts[i].Arch == arch {
			return &artifacts[i]
		}
	}
	return nil
}

// ============================================================================
// Shim management
// ============================================================================

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

	slog.Info("updated shim", "path", shimPath, "target", workspacedPath)
	return nil
}

// ============================================================================
// Helper functions
// ============================================================================

func findSourcePath() (string, bool) {
	var candidates []string

	// 1. ~/.config/workspaced/src/
	if configDir, err := env.GetConfigDir(); err == nil {
		candidates = append(candidates, filepath.Join(configDir, "src"))
	}

	// 2. $DOTFILES/workspaced/
	if dotfilesRoot, err := env.GetDotfilesRoot(); err == nil {
		candidates = append(candidates, filepath.Join(dotfilesRoot, "workspaced"))
	}

	for _, path := range candidates {
		if _, err := os.Stat(path); err == nil {
			return path, true
		}
	}

	return "", false
}

func readVersionFile(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(data)), nil
}

func getGoVersion() string {
	info, ok := debug.ReadBuildInfo()
	if !ok {
		return ""
	}

	version := info.GoVersion
	if len(version) > 2 && version[0] == 'g' && version[1] == 'o' {
		return version[2:]
	}
	return version
}

// ensureMise checks if mise exists and installs it if needed
func ensureMise(ctx context.Context) (string, error) {
	misePath := getMisePath()

	// Check if mise already exists
	if _, err := os.Stat(misePath); err == nil {
		return misePath, nil
	}

	// Mise not found, install it
	slog.Info("mise not found, installing", "path", misePath)
	if err := installMise(ctx, misePath); err != nil {
		return "", err
	}

	return misePath, nil
}

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

func installMise(ctx context.Context, misePath string) error {
	// Create directory
	if err := os.MkdirAll(filepath.Dir(misePath), 0755); err != nil {
		return fmt.Errorf("failed to create mise directory: %w", err)
	}

	slog.Info("downloading mise installer from https://mise.run")

	// Download installer
	httpClient, err := driver.Get[httpclient.Driver](ctx)
	if err != nil {
		return fmt.Errorf("failed to get http client: %w", err)
	}

	resp, err := httpClient.Client().Get("https://mise.run")
	if err != nil {
		return fmt.Errorf("failed to download installer: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("failed to download installer: HTTP %d", resp.StatusCode)
	}

	scriptBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read installer: %w", err)
	}

	// Run installer
	installCmd, err := execdriver.Run(ctx, "bash", "-s")
	if err != nil {
		return fmt.Errorf("failed to create install command: %w", err)
	}

	installCmd.Stdin = io.NopCloser(bytes.NewReader(scriptBytes))
	installCmd.Stdout = os.Stderr
	installCmd.Stderr = os.Stderr
	installCmd.Env = append(os.Environ(), fmt.Sprintf("MISE_INSTALL_PATH=%s", misePath))

	if err := installCmd.Run(); err != nil {
		return fmt.Errorf("failed to install mise: %w", err)
	}

	// Verify installation
	if _, err := os.Stat(misePath); err != nil {
		return fmt.Errorf("mise installation failed - binary not found at %s", misePath)
	}

	slog.Info("mise installed successfully", "path", misePath)
	return nil
}
