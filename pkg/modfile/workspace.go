package modfile

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"workspaced/pkg/env"
	"workspaced/pkg/git"
)

type Workspace struct {
	Root string
}

func NewWorkspace(root string) *Workspace {
	return &Workspace{Root: filepath.Clean(root)}
}

func DetectWorkspace(ctx context.Context, wd string) (*Workspace, error) {
	currentDir := wd
	if currentDir == "" {
		var err error
		currentDir, err = os.Getwd()
		if err != nil {
			return nil, err
		}
	}

	if root, err := git.GetRoot(ctx, currentDir); err == nil && root != "" {
		return NewWorkspace(root), nil
	}

	root, err := env.GetDotfilesRoot()
	if err != nil {
		return nil, fmt.Errorf("failed to detect workspace root from git and dotfiles root: %w", err)
	}
	return NewWorkspace(root), nil
}

func (w *Workspace) EnsureFiles() error {
	_, _, err := EnsureModAndSumFiles(w.Root)
	return err
}

func (w *Workspace) ModPath() string {
	return filepath.Join(w.Root, "workspaced.mod.toml")
}

func (w *Workspace) SumPath() string {
	return filepath.Join(w.Root, "workspaced.sum.toml")
}

func (w *Workspace) ModulesBaseDir() string {
	return filepath.Join(w.Root, "modules")
}
