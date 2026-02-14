package tool

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"workspaced/pkg/tool/resolution"
)

func GenerateShim(shimsDir, toolName string) error {
	shimPath := filepath.Join(shimsDir, toolName)
	workspacedPath, err := os.Executable()
	if err != nil {
		return err
	}

	// Create symlink
	os.Remove(shimPath)
	return os.Symlink(workspacedPath, shimPath)
}

func RunShim(ctx context.Context, toolName string, args []string) error {
	toolsDir, err := GetToolsDir()
	if err != nil {
		return err
	}

	resolver := resolution.NewResolver(toolsDir)
	binPath, err := resolver.Resolve(ctx, toolName)
	if err != nil {
		return fmt.Errorf("failed to resolve tool %s: %w", toolName, err)
	}

	// Exec
	cmd := exec.CommandContext(ctx, binPath, args...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	// Ensure we replace the process?
	// syscall.Exec is better if possible, but Go exec.Command is okay for now.
	// If we want to replace process, we need to handle env and args carefully.

	return cmd.Run()
}
