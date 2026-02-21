package modfile

import (
	"os"
	"path/filepath"
)

func EnsureModAndSumFiles(root string) (string, string, error) {
	modPath := filepath.Join(root, "workspaced.mod.toml")
	sumPath := filepath.Join(root, "workspaced.sum.toml")

	if _, err := os.Stat(modPath); os.IsNotExist(err) {
		if err := WriteModFile(modPath, &ModFile{
			Sources: map[string]SourceConfig{},
			Modules: map[string]string{},
		}); err != nil {
			return "", "", err
		}
	}

	if _, err := os.Stat(sumPath); os.IsNotExist(err) {
		if err := WriteSumFile(sumPath, &SumFile{
			Modules: map[string]LockedModule{},
		}); err != nil {
			return "", "", err
		}
	}

	return modPath, sumPath, nil
}
