package selfupdate

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"runtime"
	"runtime/debug"
	"strings"
	execdriver "workspaced/pkg/driver/exec"
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

	misePath, err := getMisePath()
	if err != nil {
		return fmt.Errorf("mise required to build from source: %w", err)
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
