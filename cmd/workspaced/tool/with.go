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
		DisableFlagParsing: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runWith(cmd.Context(), args)
		},
	}
}

func runWith(ctx context.Context, args []string) error {
	// Find "--" separator
	sepIdx := -1
	for i, arg := range args {
		if arg == "--" {
			sepIdx = i
			break
		}
	}

	if sepIdx == -1 {
		return fmt.Errorf("missing '--' separator\n\nUsage: workspaced tool with <tool-spec> -- <command> [args...]")
	}

	if sepIdx == 0 {
		return fmt.Errorf("missing tool spec\n\nUsage: workspaced tool with <tool-spec> -- <command> [args...]")
	}

	if sepIdx == len(args)-1 {
		return fmt.Errorf("missing command after '--'\n\nUsage: workspaced tool with <tool-spec> -- <command> [args...]")
	}

	toolSpecStr := args[0]
	cmdArgs := args[sepIdx+1:]

	// Parse tool spec: provider:package@version
	spec, err := tool.ParseToolSpec(toolSpecStr)
	if err != nil {
		return err
	}

	// Resolve binary path
	toolsDir, err := tool.GetToolsDir()
	if err != nil {
		return err
	}

	binPath, err := resolveToolBinary(toolsDir, spec, cmdArgs[0])
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

func resolveToolBinary(toolsDir string, spec tool.ToolSpec, cmdName string) (string, error) {
	// Normalize version (remove 'v' prefix)
	normalizedVersion := strings.TrimPrefix(spec.Version, "v")

	// Get version directory using spec.Dir() method
	versionDir := filepath.Join(toolsDir, spec.Dir(), normalizedVersion)

	// Check if version directory exists
	if _, err := os.Stat(versionDir); os.IsNotExist(err) {
		return "", fmt.Errorf("tool not installed: %s:%s@%s\n\nInstall it with: workspaced tool install %s:%s@%s",
			spec.Provider, spec.Package, spec.Version, spec.Provider, spec.Package, spec.Version)
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
