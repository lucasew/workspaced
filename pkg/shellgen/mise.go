package shellgen

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	execdriver "workspaced/pkg/driver/exec"
)

// GenerateMise generates mise activation code
func GenerateMise() (string, error) {
	// Use the actual mise binary path (not the wrapper)
	home := os.Getenv("HOME")
	if home == "" {
		var err error
		home, err = os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("failed to get home directory: %w", err)
		}
	}

	misePath := filepath.Join(home, ".local", "share", "workspaced", "bin", "mise")

	// Check if mise exists
	if _, err := os.Stat(misePath); os.IsNotExist(err) {
		return "", nil // Skip if mise not installed
	}

	// Execute mise activate bash using execdriver
	cmd, err := execdriver.Run(context.Background(), misePath, "activate", "bash", "--shims")
	if err != nil {
		return "", fmt.Errorf("failed to create mise command: %w", err)
	}

	cmd.Env = os.Environ()

	output, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("failed to generate mise activation: %w", err)
	}

	return string(output), nil
}
