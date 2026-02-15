package shim_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	_ "workspaced/pkg/driver/prelude"
	"workspaced/pkg/driver/shim"
)

func TestShimGeneration(t *testing.T) {
	ctx := context.Background()

	// Create temp directory for test
	tmpDir := t.TempDir()
	shimPath := filepath.Join(tmpDir, "test-shim")

	// Generate a shim that runs "echo hello world"
	err := shim.Generate(ctx, shimPath, []string{"echo", "hello", "world"})
	if err != nil {
		t.Fatalf("Failed to generate shim: %v", err)
	}

	// Verify file exists and is executable
	info, err := os.Stat(shimPath)
	if err != nil {
		t.Fatalf("Shim file not created: %v", err)
	}

	if info.Mode()&0111 == 0 {
		t.Errorf("Shim file is not executable: %o", info.Mode())
	}

	// Read shim content
	content, err := os.ReadFile(shimPath)
	if err != nil {
		t.Fatalf("Failed to read shim: %v", err)
	}

	t.Logf("Generated shim:\n%s", string(content))

	// Verify it's a bash script
	if len(content) < 2 || content[0] != '#' || content[1] != '!' {
		t.Errorf("Shim doesn't have a shebang")
	}
}

func TestShimWithSpecialCharacters(t *testing.T) {
	ctx := context.Background()

	tmpDir := t.TempDir()
	shimPath := filepath.Join(tmpDir, "special-shim")

	// Test with arguments containing spaces and special chars
	err := shim.Generate(ctx, shimPath, []string{"echo", "hello world", "$VAR", "it's test"})
	if err != nil {
		t.Fatalf("Failed to generate shim: %v", err)
	}

	content, err := os.ReadFile(shimPath)
	if err != nil {
		t.Fatalf("Failed to read shim: %v", err)
	}

	t.Logf("Generated shim with special chars:\n%s", string(content))
}
