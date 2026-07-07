package tool

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"workspaced/internal/tool/resolution"
	execdriver "workspaced/pkg/driver/exec"
)

// RunTool acts as the unified shim bridge. When the shim wrapper binary is executed
// (with its filename mimicking the tool name), this function dynamically maps that
// filename string back to the deeply nested executable within the manager's localized storage,
// configuring the process I/O streams automatically.
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
	cmd, err := execdriver.Run(ctx, binPath, args...)
	if err != nil {
		return nil, err
	}
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd, nil
}
