package modfile

import (
	"context"
	"errors"
	"fmt"
	"workspaced/pkg/configcue"
)

var (
	ErrNilConfig = errors.New("config is nil")
)

type LockResult struct {
	Sources int
}

func GenerateLock(ctx context.Context, ws *Workspace) (LockResult, error) {
	cfg, err := configcue.LoadForWorkspace(ctx, ws.Root)
	if err != nil {
		return LockResult{}, fmt.Errorf("failed to load config: %w", err)
	}
	return GenerateLockWithConfig(ctx, ws, cfg)
}

func GenerateLockWithConfig(ctx context.Context, ws *Workspace, cfg *configcue.Config) (LockResult, error) {
	if cfg == nil {
		return LockResult{}, fmt.Errorf("failed to load config: %w", ErrNilConfig)
	}

	if err := ws.EnsureFiles(ctx); err != nil {
		return LockResult{}, err
	}

	mod, err := ModFileFromConfig(cfg)
	if err != nil {
		return LockResult{}, fmt.Errorf("failed to load config: %w", err)
	}
	sourceEntries := BuildSourceLockEntries(mod)
	if err := PopulateSourceLockHashes(ctx, mod, ws.ModulesBaseDir(), sourceEntries); err != nil {
		return LockResult{}, err
	}
	_, err = ws.UpdateSumFile(ctx, func(sum *SumFile) (bool, error) {
		beforeSources := len(sum.SourceLocks())
		changed := false
		for name, entry := range sourceEntries {
			if sum.UpsertSource(name, entry) {
				changed = true
			}
		}
		afterSources := len(sum.SourceLocks())
		if afterSources < beforeSources {
			return false, fmt.Errorf("%w: before=%d after=%d", ErrLockEntryShrunk, beforeSources, afterSources)
		}
		return changed, nil
	})
	if err != nil {
		return LockResult{}, err
	}

	return LockResult{
		Sources: len(sourceEntries),
	}, nil
}
