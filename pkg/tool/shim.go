package tool

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"workspaced/pkg/tool/resolution"
)

// RunTool resolves the active version of a tool through local configuration
// (e.g., .tool-versions) or defaults to the latest installed version,
// then constructs an executable command bound to standard I/O.
// This serves as the primary gateway for shim execution and shell integration.
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
