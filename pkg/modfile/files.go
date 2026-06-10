package modfile

import (
	"context"
	"os"
	"path/filepath"
)

func EnsureLockFile(ctx context.Context, root string) (string, error) {
	sumPath := filepath.Join(root, "workspaced.lock.json")
	if _, err := os.Stat(sumPath); os.IsNotExist(err) {
		// Ensure creation goes through the shared lockfile update abstraction.
		if _, err := updateSumFile(ctx, sumPath, func(sum *SumFile) (bool, error) {
			return len(sum.Dependencies) == 0, nil
		}); err != nil {
			return "", err
		}
	}
	return sumPath, nil
}
