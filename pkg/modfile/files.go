package modfile

import (
	"os"
	"path/filepath"
)

func EnsureLockFile(root string) (string, error) {
	sumPath := filepath.Join(root, "workspaced.lock.json")
	if _, err := os.Stat(sumPath); os.IsNotExist(err) {
		if err := WriteSumFile(sumPath, &SumFile{
			Sources: map[string]LockedSource{},
		}); err != nil {
			return "", err
		}
	}
	return sumPath, nil
}
