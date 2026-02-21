package mod

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"workspaced/pkg/env"
	"workspaced/pkg/git"
)

func resolveRepoRoot() (string, error) {
	wd, err := os.Getwd()
	if err != nil {
		return "", err
	}

	if root, err := git.GetRoot(context.Background(), wd); err == nil && root != "" {
		return root, nil
	}

	if root, ok := findRepoRootFrom(wd); ok {
		return root, nil
	}

	root, err := env.GetDotfilesRoot()
	if err != nil {
		return "", fmt.Errorf("failed to detect repo root from cwd and dotfiles root: %w", err)
	}
	return root, nil
}

func findRepoRootFrom(start string) (string, bool) {
	cur := filepath.Clean(start)
	for {
		modFile := filepath.Join(cur, "workspaced.mod.toml")
		if st, err := os.Stat(modFile); err == nil && !st.IsDir() {
			return cur, true
		}

		settings := filepath.Join(cur, "settings.toml")
		modules := filepath.Join(cur, "modules")
		if st, err := os.Stat(settings); err == nil && !st.IsDir() {
			if st, err := os.Stat(modules); err == nil && st.IsDir() {
				return cur, true
			}
		}

		parent := filepath.Dir(cur)
		if parent == cur {
			break
		}
		cur = parent
	}
	return "", false
}
