package selfupdate

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"runtime/debug"
	"strings"

	"workspaced/internal/miseutil"
	"workspaced/internal/tool/backend"
	githubprov "workspaced/internal/tool/backend/github"
	"workspaced/internal/version"
	envdriver "workspaced/pkg/driver/env"
	execdriver "workspaced/pkg/driver/exec"
	"workspaced/pkg/driver/shim"
	"workspaced/pkg/logging"
	"workspaced/pkg/taskgroup"

	"github.com/spf13/cobra"
)

var (
	ErrGoVersionUnknown     = errors.New("could not determine Go version from build info")
	ErrNoVersionsFound      = errors.New("no versions found")
	ErrArtifactToolRequired = errors.New("github tool does not support ArtifactTool (needed for selfupdate)")
	ErrNoArtifactFound      = errors.New("no artifact found for current platform")
	ErrNoBinaryFound        = errors.New("no binary found")
)

func GetCommand() *cobra.Command {
	var force bool

	cmd := &cobra.Command{
		Use:   "self-update",
		Short: "Update workspaced binary",
		Long: `Update workspaced to the latest version.

Strategy:
  1. If source code exists → rebuild from source (always)
  2. Otherwise → download from GitHub using the tool backend

The update is installed in ~/.local/share/workspaced/tools/ and the shim
in ~/.local/bin/workspaced is updated automatically.`,
		RunE: func(c *cobra.Command, args []string) error {
			ctx := c.Context()
			g := taskgroup.FromContext(ctx)

			// Choose the right pool for the self-update task itself so it
			// participates in concurrency limits and gets the correct
			// emoji/type in the progress UI.
			pool := taskgroup.Internet
			msg := "downloading from GitHub"
			srcPath, err := findSourcePath(ctx)
			if err != nil {
				return err
			}
			if srcPath != "" {
				pool = taskgroup.CPU // compiling is CPU-bound
				msg = "compiling from source"
			}

			g.Go("self-update", pool, func(ctx context.Context, s *taskgroup.Status) error {
				s.Update(msg)
				defer s.Unit()()
				return runSelfUpdate(ctx, force)
			})

			return nil
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
	srcPath, err := findSourcePath(ctx)
	if err != nil {
		return err
	}
	if srcPath != "" {
		logger := logging.GetLogger(ctx)
		logger.Info("building from source (always rebuilds)", "path", srcPath)
		return buildAndInstallFromSource(ctx, srcPath)
	}

	// Fallback to GitHub provider (checks version unless --force)
	return updateFromGitHub(ctx, force)
}

// ============================================================================
// Source build strategy
// ============================================================================

func buildAndInstallFromSource(ctx context.Context, srcPath string) error {
	// Install to fixed location (not versioned)
	home, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("failed to get home directory: %w", err)
	}

	installDir := filepath.Join(home, ".local", "share", "workspaced", "bin")
	installPath := filepath.Join(installDir, "workspaced")

	goVersion := getGoVersion()
	if goVersion == "" {
		return ErrGoVersionUnknown
	}

	misePath, err := miseutil.Ensure(ctx)
	if err != nil {
		return fmt.Errorf("ensure mise: %w", err)
	}

	if err := os.MkdirAll(installDir, 0755); err != nil {
		return err
	}
	tmpOut, err := os.CreateTemp(installDir, "workspaced.tmp-*")
	if err != nil {
		return err
	}
	tmpPath := tmpOut.Name()
	if err := tmpOut.Close(); err != nil {
		return err
	}
	defer logging.RunCleanup(ctx, "remove", func() error {
		if err := os.Remove(tmpPath); err != nil && !errors.Is(err, os.ErrNotExist) {
			return err
		}
		return nil
	}, "path", tmpPath)

	goSpec := fmt.Sprintf("go@%s", goVersion)
	buildCmd, err := execdriver.Run(ctx, misePath, "exec", goSpec, "--",
		"go", "build", "-v", "-o", tmpPath, "./cmd/workspaced")
	if err != nil {
		return err
	}

	buildCmd.Dir = srcPath
	buildCmd.Stdout = os.Stdout
	buildCmd.Stderr = os.Stderr

	logger := logging.GetLogger(ctx)
	logger.Info("building from source", "path", srcPath, "go", goSpec)
	if err := buildCmd.Run(); err != nil {
		return fmt.Errorf("build failed: %w", err)
	}
	if err := os.Chmod(tmpPath, 0755); err != nil {
		return fmt.Errorf("failed to set permissions on built binary: %w", err)
	}
	if err := os.Rename(tmpPath, installPath); err != nil {
		return fmt.Errorf("failed to install built binary: %w", err)
	}

	logger.Info("build completed", "path", installPath)
	return updateWorkspacedShim(ctx, installPath)
}

// ============================================================================
// GitHub provider strategy
// ============================================================================

func updateFromGitHub(ctx context.Context, force bool) error {
	// Use the exposed constructor directly. This works even without the old
	// detailed methods on the thin Provider interface, and demonstrates how
	// a future registry backend (or other code) can obtain a github Tool.
	t, err := githubprov.NewTool("lucasew/workspaced")
	if err != nil {
		return err
	}

	// Get latest version via the Tool
	versions, err := t.ListVersions(ctx)
	if err != nil {
		return err
	}

	if len(versions) == 0 {
		return ErrNoVersionsFound
	}

	latestVersion := versions[0]
	normalizedLatest := strings.TrimPrefix(latestVersion, "v")

	// Check if update is needed (unless --force)
	if !force {
		currentVersion := version.Version()

		if currentVersion == normalizedLatest {
			logger := logging.GetLogger(ctx)
			logger.Info("already at latest version", "version", currentVersion)
			return nil
		}

		logger := logging.GetLogger(ctx)
		logger.Info("updating", "current", currentVersion, "latest", normalizedLatest)
	} else {
		logger := logging.GetLogger(ctx)
		logger.Info("forcing update", "version", latestVersion)
	}

	// Use ArtifactTool + the shared SelectArtifact for platform selection.
	at, ok := t.(backend.ArtifactTool)
	if !ok {
		return ErrArtifactToolRequired
	}

	artifacts, err := at.ListArtifacts(ctx, latestVersion)
	if err != nil {
		return err
	}

	// Standard platform selection (same logic used by tool installs etc.).
	// The "workspaced" hint helps disambiguate when a release has multiple
	// assets for the same OS/arch.
	artifact := backend.SelectArtifact(artifacts, runtime.GOOS, runtime.GOARCH, "workspaced")
	if artifact == nil {
		available := []string{}
		for _, a := range artifacts {
			available = append(available, fmt.Sprintf("%s/%s", a.OS, a.Arch))
		}
		return fmt.Errorf("%w: %s/%s (available: %v)", ErrNoArtifactFound, runtime.GOOS, runtime.GOARCH, available)
	}

	// Install to fixed location (not versioned)
	home, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("failed to get home directory: %w", err)
	}

	installDir := filepath.Join(home, ".local", "share", "workspaced", "bin")
	tmpDir := filepath.Join(installDir, ".tmp-"+normalizedLatest)

	if err := os.MkdirAll(tmpDir, 0755); err != nil {
		return err
	}
	defer logging.RunCleanup(ctx, "remove_all", func() error { return os.RemoveAll(tmpDir) }, "path", tmpDir)

	logger := logging.GetLogger(ctx)
	logger.Info("downloading from GitHub", "version", latestVersion, "os", artifact.OS, "arch", artifact.Arch)
	if err := at.InstallArtifact(ctx, *artifact, tmpDir); err != nil {
		return fmt.Errorf("installation failed: %w", err)
	}

	workspacedBin, err := findBinary(tmpDir)
	if err != nil {
		return fmt.Errorf("workspaced binary not found in downloaded archive: %w", err)
	}

	targetName := "workspaced"
	if runtime.GOOS == "windows" {
		targetName = "workspaced.exe"
	}
	installPath := filepath.Join(installDir, targetName)
	if err := os.MkdirAll(installDir, 0755); err != nil {
		return err
	}

	if err := os.Rename(workspacedBin, installPath); err != nil {
		return fmt.Errorf("failed to install binary: %w", err)
	}

	if err := os.Chmod(installPath, 0755); err != nil {
		return fmt.Errorf("failed to set permissions: %w", err)
	}

	logger.Info("download completed", "path", installPath)
	return updateWorkspacedShim(ctx, installPath)
}

func findBinary(dir string) (string, error) {
	// 1. Check standard names (strict first)
	targets := []string{"workspaced", "workspaced.exe"}
	for _, t := range targets {
		path := filepath.Join(dir, t)
		if _, err := os.Stat(path); err == nil {
			return path, nil
		}
		// Check bin/ subdirectory
		path = filepath.Join(dir, "bin", t)
		if _, err := os.Stat(path); err == nil {
			return path, nil
		}
	}

	// 2. Scan for any executable-looking file
	// This fallback handles cases where:
	// - Binary has a suffix (e.g. workspaced-linux-amd64) and extraction didn't rename it
	// - Binary name is different from expected
	entries, err := os.ReadDir(dir)
	if err != nil {
		return "", err
	}

	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		// Ignore common non-binary files
		if strings.HasPrefix(name, ".") ||
			strings.HasSuffix(name, ".sha256") ||
			strings.HasSuffix(name, ".md") ||
			strings.HasSuffix(name, ".txt") ||
			name == "LICENSE" {
			continue
		}

		// On Unix, check executable bit
		if runtime.GOOS != "windows" {
			info, err := e.Info()
			if err == nil && info.Mode()&0111 != 0 {
				return filepath.Join(dir, name), nil
			}
		} else {
			// On Windows, check extension
			if strings.HasSuffix(strings.ToLower(name), ".exe") {
				return filepath.Join(dir, name), nil
			}
		}
	}

	// 3. Last resort: if there is only one file and it's not excluded above, pick it
	// This covers Linux binaries that might not have +x set yet (though they should)
	var candidates []string
	for _, e := range entries {
		if !e.IsDir() {
			name := e.Name()
			if !strings.HasPrefix(name, ".") &&
				!strings.HasSuffix(name, ".sha256") &&
				!strings.HasSuffix(name, ".md") &&
				!strings.HasSuffix(name, ".txt") &&
				name != "LICENSE" {
				candidates = append(candidates, filepath.Join(dir, name))
			}
		}
	}

	if len(candidates) == 1 {
		return candidates[0], nil
	}

	return "", fmt.Errorf("%w: %s", ErrNoBinaryFound, dir)
}

func updateWorkspacedShim(ctx context.Context, workspacedPath string) error {
	shimPath, err := shim.GenerateInLocalBin(ctx, "workspaced", []string{workspacedPath})
	if err != nil {
		return err
	}
	logging.GetLogger(ctx).Info("updated shim", "path", shimPath, "target", workspacedPath)
	return nil
}

func findSourcePath(ctx context.Context) (string, error) {
	var candidates []string

	// 1. ~/.config/workspaced/src/
	configDir, err := envdriver.GetConfigDir(ctx)
	if err != nil {
		return "", fmt.Errorf("config dir: %w", err)
	}
	candidates = append(candidates, filepath.Join(configDir, "src"))

	// 2. $DOTFILES/workspaced/
	dotfilesRoot, err := envdriver.GetDotfilesRoot(ctx)
	if err != nil {
		return "", fmt.Errorf("dotfiles root: %w", err)
	}
	candidates = append(candidates, filepath.Join(dotfilesRoot, "workspaced"))

	for _, path := range candidates {
		if _, err := os.Stat(path); err == nil {
			return path, nil
		}
	}

	// No candidate exists on disk; caller falls back to GitHub update.
	return "", nil
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
