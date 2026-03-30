package modfile

import (
	"context"
	"fmt"
	"workspaced/pkg/configcue"
)

type LockResult struct {
	Sources int
}

func GenerateLock(ctx context.Context, ws *Workspace) (LockResult, error) {
	cfg, err := configcue.LoadForWorkspace(ws.Root)
	if err != nil {
		return LockResult{}, fmt.Errorf("failed to load config: %w", err)
	}
	return GenerateLockWithConfig(ctx, ws, cfg)
}

func GenerateLockWithConfig(ctx context.Context, ws *Workspace, cfg *configcue.Config) (LockResult, error) {
	if cfg == nil {
		return LockResult{}, fmt.Errorf("failed to load config: config is nil")
	}

	if err := ws.EnsureFiles(); err != nil {
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
	_, err = UpdateSumFile(ws.SumPath(), func(sum *SumFile) (bool, error) {
		beforeSources := len(sum.Sources)
		changed := false
		for name, entry := range sourceEntries {
			if sum.UpsertSource(name, entry) {
				changed = true
			}
		}
		if len(sum.Sources) < beforeSources {
			return false, fmt.Errorf("refusing to shrink source lock entries: before=%d after=%d", beforeSources, len(sum.Sources))
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
