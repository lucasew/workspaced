package modfile

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	envdriver "workspaced/pkg/driver/env"
	"workspaced/internal/git"
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

	root, err := envdriver.GetDotfilesRoot(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to detect workspace root from git and dotfiles root: %w", err)
	}
	return NewWorkspace(root), nil
}

func (w *Workspace) EnsureFiles(ctx context.Context) error {
	_, err := EnsureLockFile(ctx, w.Root)
	return err
}

func (w *Workspace) LoadSumFile() (*SumFile, error) {
	return LoadSumFile(w.SumPath())
}

func (w *Workspace) UpdateSumFile(ctx context.Context, mutate func(sum *SumFile) (bool, error)) (bool, error) {
	return updateSumFile(ctx, w.SumPath(), mutate)
}

func (w *Workspace) SumPath() string {
	return filepath.Join(w.Root, "workspaced.lock.json")
}

func (w *Workspace) ModulesBaseDir() string {
	return filepath.Join(w.Root, "modules")
}
