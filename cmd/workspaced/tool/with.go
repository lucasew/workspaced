package tool

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"workspaced/pkg/tool"

	"github.com/spf13/cobra"
)

func newWithCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "with <tool-spec> -- <command> [args...]",
		Short: "Execute a command with a specific tool version",
		Long: `Execute a command with a specific tool version.

If the tool is not installed, it will be installed automatically.

Tool spec format:
  provider:package@version  (full spec)
  provider:package          (uses latest version)
  package@version           (uses github provider)
  package                   (uses github provider and latest version)

Examples:
  workspaced tool with github:denoland/deno@1.40.0 -- deno run app.ts
  workspaced tool with denoland/deno -- deno --version
  workspaced tool with deno@1.40.0 -- deno run app.ts
  workspaced tool with ripgrep -- rg pattern`,
		Args: cobra.MinimumNArgs(2), // Need at least: tool-spec and command
		RunE: func(cmd *cobra.Command, args []string) error {
			return runWith(cmd.Context(), args)
		},
	}
}

func runWith(ctx context.Context, args []string) error {
	// Cobra processes flags and consumes "--" automatically
	// So we receive: [tool-spec, command, arg1, arg2, ...]
	// The "--" is just visual sugar for the user, we don't depend on it

	toolSpecStr := args[0]
	cmdArgs := args[1:] // Everything after spec is the command

	// Parse tool spec: provider:package@version
	spec, err := tool.ParseToolSpec(toolSpecStr)
	if err != nil {
		return err
	}

	// Resolve binary path (auto-install if needed)
	binPath, err := resolveOrInstallTool(ctx, spec, cmdArgs[0])
	if err != nil {
		return err
	}

	// Execute command
	command := exec.CommandContext(ctx, binPath, cmdArgs[1:]...)
	command.Stdin = os.Stdin
	command.Stdout = os.Stdout
	command.Stderr = os.Stderr

	return command.Run()
}

func resolveLatestVersion(ctx context.Context, spec tool.ToolSpec) (string, error) {
	provider, err := tool.GetProvider(spec.Provider)
	if err != nil {
		return "", err
	}

	pkgConfig, err := provider.ParsePackage(spec.Package)
	if err != nil {
		return "", err
	}

	versions, err := provider.ListVersions(ctx, pkgConfig)
	if err != nil {
		return "", fmt.Errorf("failed to list versions: %w", err)
	}

	if len(versions) == 0 {
		return "", fmt.Errorf("no versions found for %s", spec)
	}

	// Return first version (assumed to be latest)
	return versions[0], nil
}

func resolveOrInstallTool(ctx context.Context, spec tool.ToolSpec, cmdName string) (string, error) {
	toolsDir, err := tool.GetToolsDir()
	if err != nil {
		return "", err
	}

	// If version is "latest", resolve to actual version first
	actualVersion := spec.Version
	if spec.Version == "latest" {
		resolved, err := resolveLatestVersion(ctx, spec)
		if err != nil {
			return "", fmt.Errorf("failed to resolve latest version: %w", err)
		}
		actualVersion = resolved
		// Update spec with resolved version
		spec.Version = actualVersion
	}

	// Try to resolve the binary first
	binPath, err := resolveToolBinary(toolsDir, spec, cmdName)
	if err == nil {
		return binPath, nil
	}

	// If not found, check if it's because the tool isn't installed
	normalizedVersion := strings.TrimPrefix(actualVersion, "v")
	versionDir := filepath.Join(toolsDir, spec.Dir(), normalizedVersion)

	if _, statErr := os.Stat(versionDir); os.IsNotExist(statErr) {
		// Tool not installed, install it automatically
		fmt.Fprintf(os.Stderr, "Tool not installed: %s\n", spec)
		fmt.Fprintf(os.Stderr, "Installing automatically...\n")

		manager, err := tool.NewManager()
		if err != nil {
			return "", fmt.Errorf("failed to create manager: %w", err)
		}

		if err := manager.Install(ctx, spec.String()); err != nil {
			return "", fmt.Errorf("failed to install tool: %w", err)
		}

		fmt.Fprintf(os.Stderr, "Installation complete!\n\n")

		// Try to resolve again after installation
		binPath, err = resolveToolBinary(toolsDir, spec, cmdName)
		if err != nil {
			return "", fmt.Errorf("tool installed but binary not found: %w", err)
		}
		return binPath, nil
	}

	// Tool is installed but binary not found (different error)
	return "", err
}

func resolveToolBinary(toolsDir string, spec tool.ToolSpec, cmdName string) (string, error) {
	// Normalize version (remove 'v' prefix)
	normalizedVersion := strings.TrimPrefix(spec.Version, "v")

	// Get version directory using spec.Dir() method
	versionDir := filepath.Join(toolsDir, spec.Dir(), normalizedVersion)

	// Check if version directory exists
	if _, err := os.Stat(versionDir); os.IsNotExist(err) {
		return "", fmt.Errorf("tool directory not found: %s", versionDir)
	}

	// Look for binary in common locations
	candidates := []string{
		filepath.Join(versionDir, "bin", cmdName),
		filepath.Join(versionDir, "bin", cmdName+".exe"),
		filepath.Join(versionDir, cmdName),
		filepath.Join(versionDir, cmdName+".exe"),
	}

	for _, path := range candidates {
		if _, err := os.Stat(path); err == nil {
			return path, nil
		}
	}

	return "", fmt.Errorf("binary %q not found in %s", cmdName, versionDir)
}
