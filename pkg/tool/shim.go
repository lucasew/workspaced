package tool

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"workspaced/pkg/tool/resolution"
)

// RunTool executes a managed tool by name with the given arguments.
// Used by shell integration and direct invocation.
func RunTool(ctx context.Context, toolName string, args ...string) (*exec.Cmd, error) {
	toolsDir, err := GetToolsDir()
	if err != nil {
		return nil, err
	}

	resolver := resolution.NewResolver(toolsDir)
	binPath, err := resolver.Resolve(ctx, toolName)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve tool %s: %w", toolName, err)
	}

	// Exec
	cmd := exec.CommandContext(ctx, binPath, args...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd, nil
}
